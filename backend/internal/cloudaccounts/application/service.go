// Package application holds the cloud-connection use-cases and the sync pipeline entry point.
package application

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/hibiken/asynq"

	"github.com/quixgit/greenops-reliabilix/backend/internal/cloudaccounts/domain"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/focus"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/observability"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/queue"
)

// Ingestor turns raw provider batches into FOCUS usage records (implemented by the ingestion module).
type Ingestor interface {
	Ingest(ctx context.Context, tenantID, projectID, connectionID string, batches []focus.RawBatch) (focus.IngestResult, error)
}

// Setup is what the customer needs to create the cross-account role.
type Setup struct {
	// ExternalID is the connection's ownership token: the AWS ExternalId, or the label key the customer puts on
	// the BigQuery dataset.
	ExternalID          string         `json:"external_id"`
	PlatformAccountID   string         `json:"platform_account_id,omitempty"`
	TrustPolicy         map[string]any `json:"trust_policy,omitempty"`
	PermissionsPolicy   map[string]any `json:"permissions_policy"`
	RequiredPermissions []string       `json:"required_permissions"`
	// Steps are the human-readable instructions, in order (GCP: prerequisites, export, label, access).
	Steps []string `json:"steps,omitempty"`
}

type Service struct {
	Repo      domain.Repository
	Providers *domain.Registry
	Ingestor  Ingestor
	// Rightsizing is optional: without it rightsizing jobs are no-ops.
	Rightsizing domain.RightsizingSink
	Queue       queue.Enqueuer
	Log         *slog.Logger

	PlatformAWSAccountID string
	// PlatformGCPServiceAccount is the principal GCP customers grant read access to their billing dataset.
	PlatformGCPServiceAccount string
	BackfillDays              int // first sync window, capped at 365
	Now                       func() time.Time
}

// overlap re-fetches the most recent days on every sync: cloud billing is adjusted retroactively.
const overlapDays = 7

func (s Service) now() time.Time {
	if s.Now != nil {
		return s.Now().UTC()
	}
	return time.Now().UTC()
}

func (s Service) List(ctx context.Context, tenantID string) ([]domain.Connection, error) {
	return s.Repo.List(ctx, tenantID)
}

func (s Service) Get(ctx context.Context, tenantID, id string) (domain.Connection, error) {
	return s.Repo.Get(ctx, tenantID, id)
}

func (s Service) Runs(ctx context.Context, tenantID, id string) ([]domain.SyncRun, error) {
	if _, err := s.Repo.Get(ctx, tenantID, id); err != nil {
		return nil, err
	}
	return s.Repo.ListRuns(ctx, tenantID, id)
}

func (s Service) Delete(ctx context.Context, tenantID, id string) error {
	return s.Repo.Delete(ctx, tenantID, id)
}

// Connect registers a pending connection and returns the role setup instructions. It does not call
// the cloud: the customer first creates the role with the shown ExternalId, then calls Verify.
func (s Service) Connect(ctx context.Context, c domain.Connection) (domain.Connection, Setup, error) {
	if err := c.Validate(); err != nil {
		return domain.Connection{}, Setup{}, err
	}
	if _, err := s.Providers.Get(c.Provider); err != nil {
		return domain.Connection{}, Setup{}, err
	}
	ext, err := domain.NewExternalID()
	if err != nil {
		return domain.Connection{}, Setup{}, err
	}
	c.ExternalID = ext
	created, err := s.Repo.Create(ctx, c)
	if err != nil {
		return domain.Connection{}, Setup{}, err
	}
	return created, s.setup(created), nil
}

func (s Service) setup(c domain.Connection) Setup {
	if c.Provider == domain.GCP {
		return s.gcpSetup(c)
	}
	st := Setup{
		ExternalID:        c.ExternalID,
		PlatformAccountID: s.PlatformAWSAccountID,
		// GetRightsizingRecommendation is optional: without it only the rightsizing advice is missing.
		RequiredPermissions: []string{"ce:GetCostAndUsage", "ce:GetRightsizingRecommendation"},
		PermissionsPolicy: map[string]any{"Version": "2012-10-17", "Statement": []map[string]any{
			{"Effect": "Allow", "Action": []string{"ce:GetCostAndUsage", "ce:GetRightsizingRecommendation"}, "Resource": "*"}}},
	}
	if s.PlatformAWSAccountID != "" {
		st.TrustPolicy = map[string]any{"Version": "2012-10-17", "Statement": []map[string]any{{
			"Effect": "Allow", "Action": "sts:AssumeRole",
			"Principal": map[string]any{"AWS": "arn:aws:iam::" + s.PlatformAWSAccountID + ":root"},
			"Condition": map[string]any{"StringEquals": map[string]any{"sts:ExternalId": c.ExternalID}},
		}}}
	}
	return st
}

// gcpSetup explains how a customer connects their Cloud Billing export. Nothing here is a secret: the label key
// is the connection's ownership token and the member is the platform's public service account email.
func (s Service) gcpSetup(c domain.Connection) Setup {
	t, err := domain.ParseBigQueryRef(c.CredentialRef)
	if err != nil { // validated on creation; defensive
		return Setup{ExternalID: c.ExternalID, RequiredPermissions: []string{}, PermissionsPolicy: map[string]any{}}
	}
	member := s.PlatformGCPServiceAccount
	st := Setup{
		ExternalID:          c.ExternalID,
		PlatformAccountID:   member,
		RequiredPermissions: []string{"roles/bigquery.dataViewer on the billing export dataset (read-only)"},
		PermissionsPolicy: map[string]any{
			"dataset":       t.Project + ":" + t.Dataset,
			"dataset_label": map[string]string{"key": c.ExternalID, "value": "1"},
			"dataset_iam":   map[string]string{"role": "roles/bigquery.dataViewer", "member": "serviceAccount:" + member},
		},
	}
	st.Steps = []string{
		fmt.Sprintf("Enable the BigQuery API in project %s: gcloud services enable bigquery.googleapis.com --project %s", t.Project, t.Project),
		"Turn on Cloud Billing export to BigQuery (Billing > Billing export > BigQuery export > Standard usage cost) into the dataset " + t.Dataset +
			". This needs the Billing Account Administrator role. Data is exported from the day you enable it (there is no history), and the first delivery can take up to 48 hours.",
		fmt.Sprintf("Prove you control the dataset by adding this label: bq update --set_label %s:1 %s:%s", c.ExternalID, t.Project, t.Dataset),
	}
	if member != "" {
		st.Steps = append(st.Steps,
			fmt.Sprintf("Share the dataset with serviceAccount:%s as BigQuery Data Viewer (read-only); deploy/gcp/customer-access.tf does steps 3 and 4 in Terraform.", member))
	} else {
		st.Steps = append(st.Steps, "Contact support for the service account to share the dataset with (it is not configured on this platform yet).")
	}
	st.Steps = append(st.Steps, "Verify the connection. Nothing else is shared: the platform reads only the billing export table of the dataset.")
	return st
}

// SetupFor returns the setup instructions of an existing connection.
func (s Service) SetupFor(ctx context.Context, tenantID, id string) (Setup, error) {
	c, err := s.Repo.Get(ctx, tenantID, id)
	if err != nil {
		return Setup{}, err
	}
	return s.setup(c), nil
}

// SetExport points an AWS connection at the customer's FOCUS data export (nil switches back to Cost Explorer).
// The connection becomes pending: the customer verifies again, which also proves the role can read the export.
func (s Service) SetExport(ctx context.Context, tenantID, id string, e *domain.ExportConfig) (domain.Connection, error) {
	c, err := s.Repo.Get(ctx, tenantID, id)
	if err != nil {
		return domain.Connection{}, err
	}
	if c.Provider != domain.AWS {
		return domain.Connection{}, fmt.Errorf("%w: data exports are supported for AWS only", domain.ErrInvalidConnection)
	}
	if e != nil {
		if err := e.Validate(); err != nil {
			return domain.Connection{}, err
		}
	}
	if err := s.Repo.SetExport(ctx, tenantID, id, e); err != nil {
		return domain.Connection{}, err
	}
	return s.Repo.Get(ctx, tenantID, id)
}

// Verify checks the granted access. On success the connection becomes healthy and a first sync is queued.
func (s Service) Verify(ctx context.Context, tenantID, id string) (domain.Connection, error) {
	c, err := s.Repo.Get(ctx, tenantID, id)
	if err != nil {
		return domain.Connection{}, err
	}
	p, err := s.Providers.Get(c.Provider)
	if err != nil {
		return domain.Connection{}, err
	}
	if err := p.Validate(ctx, c); err != nil {
		msg := safeMessage(err)
		_ = s.Repo.SetStatus(ctx, tenantID, id, domain.StatusError, &msg)
		_ = s.Repo.Audit(ctx, tenantID, "cloud_connection.verification_failed", id, map[string]any{"reason": msg})
		return domain.Connection{}, err
	}
	if err := s.Repo.SetStatus(ctx, tenantID, id, domain.StatusHealthy, nil); err != nil {
		return domain.Connection{}, err
	}
	_ = s.Repo.Audit(ctx, tenantID, "cloud_connection.verified", id, nil)
	if err := s.SyncNow(ctx, tenantID, c.ProjectID, id); err != nil {
		return domain.Connection{}, err
	}
	return s.Repo.Get(ctx, tenantID, id)
}

// SyncNow queues a sync (deduplicated while one is pending).
func (s Service) SyncNow(ctx context.Context, tenantID, projectID, connectionID string) error {
	return s.Queue.Enqueue(ctx, queue.TaskSyncAWSAccount,
		queue.TenantPayload{TenantID: tenantID, ProjectID: projectID, RefID: connectionID},
		asynq.Unique(10*time.Minute))
}

// FanOut queues one sync per syncable connection across all tenants (scheduler job).
func (s Service) FanOut(ctx context.Context) (int, error) {
	refs, err := s.Repo.ListForSync(ctx)
	if err != nil {
		return 0, err
	}
	n := 0
	for _, j := range refs {
		if err := s.SyncNow(ctx, j.TenantID, j.ProjectID, j.ConnectionID); err != nil && !errors.Is(err, asynq.ErrDuplicateTask) {
			s.Log.Error("enqueue sync", "connection", j.ConnectionID, "err", err)
			continue
		}
		n++
	}
	return n, nil
}

// RightsizingNow queues a rightsizing refresh (deduplicated while one is pending).
func (s Service) RightsizingNow(ctx context.Context, tenantID, projectID, connectionID string) error {
	return s.Queue.Enqueue(ctx, queue.TaskSyncRightsizing,
		queue.TenantPayload{TenantID: tenantID, ProjectID: projectID, RefID: connectionID},
		asynq.Unique(time.Hour))
}

// FanOutRightsizing queues one rightsizing refresh per syncable connection across all tenants (scheduler job).
func (s Service) FanOutRightsizing(ctx context.Context) (int, error) {
	refs, err := s.Repo.ListForSync(ctx)
	if err != nil {
		return 0, err
	}
	n := 0
	for _, j := range refs {
		if err := s.RightsizingNow(ctx, j.TenantID, j.ProjectID, j.ConnectionID); err != nil && !errors.Is(err, asynq.ErrDuplicateTask) {
			s.Log.Error("enqueue rightsizing", "connection", j.ConnectionID, "err", err)
			continue
		}
		n++
	}
	return n, nil
}

// RunRightsizing fetches the provider's rightsizing findings for one connection and hands them to the
// recommendations module. It is best effort: a connection whose role lacks the optional permission simply has
// no findings, and that must never mark the connection unhealthy (cost syncs are unaffected).
func (s Service) RunRightsizing(ctx context.Context, tenantID, connectionID string) error {
	if s.Rightsizing == nil {
		return nil
	}
	c, err := s.Repo.Get(ctx, tenantID, connectionID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return nil
		}
		return err
	}
	p, err := s.Providers.Get(c.Provider)
	if err != nil {
		return err
	}
	rp, ok := p.(domain.RightsizingProvider)
	if !ok {
		return nil
	}
	findings, err := rp.GetRightsizing(ctx, c)
	if err != nil {
		if errors.Is(err, domain.ErrAccessDenied) {
			s.Log.Info("rightsizing not permitted for this connection; add ce:GetRightsizingRecommendation to the role", "connection", connectionID)
			return nil
		}
		return err
	}
	return s.Rightsizing.SubmitRightsizing(ctx, tenantID, c.ProjectID, findings)
}

// window computes the [from, to) days to fetch: incremental with overlap, or the initial backfill.
func (s Service) window(c domain.Connection) (from, to time.Time) {
	to = s.now().Truncate(24 * time.Hour) // today is excluded: its cost is still estimated
	days := s.BackfillDays
	if days <= 0 || days > 365 {
		days = 60
	}
	from = to.AddDate(0, 0, -days)
	if c.SyncedThrough != nil {
		from = c.SyncedThrough.UTC().AddDate(0, 0, -(overlapDays - 1))
		if min := to.AddDate(0, 0, -365); from.Before(min) {
			from = min
		}
	}
	return from, to
}

// RunSync is the sync_aws_account job: fetch -> ingest (normalize to FOCUS, persist) -> mark synced -> queue
// carbon calculation. Returns domain.ErrAccessDenied (not retryable) when the customer's role stopped working.
func (s Service) RunSync(ctx context.Context, tenantID, connectionID string) error {
	c, err := s.Repo.Get(ctx, tenantID, connectionID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return nil // connection deleted while the job waited
		}
		return err
	}
	p, err := s.Providers.Get(c.Provider)
	if err != nil {
		return err
	}
	runID, err := s.Repo.StartRun(ctx, tenantID, connectionID)
	if err != nil {
		return err
	}
	start := time.Now()
	label := string(c.Provider)
	fail := func(cause error) error {
		msg := safeMessage(cause)
		_ = s.Repo.FinishRun(ctx, tenantID, runID, false, 0, &msg)
		if domain.NeedsCustomerAction(cause) {
			_ = s.Repo.SetStatus(ctx, tenantID, connectionID, domain.StatusError, &msg)
			observability.CloudSyncTotal.WithLabelValues(label, "denied").Inc()
		} else {
			observability.CloudSyncTotal.WithLabelValues(label, "failed").Inc()
		}
		observability.CloudSyncDuration.WithLabelValues(label).Observe(time.Since(start).Seconds())
		return cause
	}

	from, to := s.window(c)
	var res focus.IngestResult
	if from.Before(to) {
		batches, err := p.GetUsage(ctx, domain.UsageRequest{Connection: c, From: from, To: to})
		if err != nil {
			return fail(err)
		}
		if res, err = s.Ingestor.Ingest(ctx, tenantID, c.ProjectID, connectionID, batches); err != nil {
			return fail(fmt.Errorf("ingest: %w", err))
		}
	}
	if err := s.Repo.MarkSynced(ctx, tenantID, connectionID, to.AddDate(0, 0, -1), res.Records); err != nil {
		return fail(err)
	}
	if err := s.Repo.FinishRun(ctx, tenantID, runID, true, res.Records, nil); err != nil {
		return err
	}
	observability.CloudSyncTotal.WithLabelValues(label, "succeeded").Inc()
	observability.CloudSyncDuration.WithLabelValues(label).Observe(time.Since(start).Seconds())

	if res.Records > 0 { // chain: a finished sync queues the carbon calculation for the touched days
		return s.Queue.Enqueue(ctx, queue.TaskCalculateCarbon, queue.WindowPayload{
			TenantID: tenantID, ProjectID: c.ProjectID,
			From: res.From.Format(time.DateOnly), To: res.To.AddDate(0, 0, 1).Format(time.DateOnly),
		})
	}
	return nil
}

// safeMessage is stored and shown to users: it never contains provider internals or secrets.
func safeMessage(err error) string {
	switch {
	case errors.Is(err, domain.ErrAccessDenied):
		return "Access denied: check the access you granted (AWS role and its read permissions, or the platform's read access to the BigQuery dataset)."
	case errors.Is(err, domain.ErrBigQueryDisabled):
		return "BigQuery is not enabled in your Google Cloud project. Enable the BigQuery API, turn on Cloud Billing export to BigQuery, then verify again."
	case errors.Is(err, domain.ErrOwnershipNotProven):
		return "Ownership of the billing dataset is not proven: add the connection's label to the dataset (see the setup steps), then verify again."
	case errors.Is(err, domain.ErrExportNotFound):
		return "Billing export not found: check that the export is enabled, delivered (the first delivery can take up to 48 hours) and that the location is correct, then verify again."
	default:
		return "Sync failed; it will be retried automatically."
	}
}
