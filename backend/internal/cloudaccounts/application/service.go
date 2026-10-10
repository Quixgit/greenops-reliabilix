// Package application holds the cloud-connection use-cases and the sync pipeline entry point.
package application

import (
	"context"
	"errors"
	"fmt"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/entitlements"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/settings"
	"log/slog"
	"slices"
	"strings"
	"time"

	"github.com/hibiken/asynq"

	"github.com/quixgit/greenops-reliabilix/backend/internal/cloudaccounts/domain"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/focus"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/notify"
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
	// Settings and Limits are optional (catalog defaults, no limits when nil): allowed providers, connection limit and
	// the minimum interval between data pulls come from the organization's settings (ADR-0023).
	Settings settings.Reader
	Limits   entitlements.Gate
	// Notify is optional: a connection that starts to need customer action is announced (best effort).
	Notify notify.Notifier
	Log    *slog.Logger

	PlatformAWSAccountID string
	// PlatformGCPServiceAccount is the principal GCP customers grant read access to their billing dataset.
	PlatformGCPServiceAccount string
	// PlatformAzureAppID is the application (client) id of the platform's Microsoft Entra application: the principal
	// Azure customers create in their tenant and grant read access to their export container.
	PlatformAzureAppID string
	BackfillDays       int // first sync window, capped at 365
	Now                func() time.Time
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
	if s.Settings != nil && !slices.Contains(s.Settings.Strings(ctx, c.TenantID, settings.KeyProvidersAllowed), string(c.Provider)) {
		return domain.Connection{}, Setup{}, domain.ErrProviderNotAllowed
	}
	if s.Limits != nil {
		if err := s.Limits.Allow(ctx, c.TenantID, entitlements.Connections); err != nil {
			return domain.Connection{}, Setup{}, err
		}
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
	switch c.Provider {
	case domain.GCP:
		return s.gcpSetup(c)
	case domain.Azure:
		return s.azureSetup(c)
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

// azureSetup explains how a customer connects their Cost Management export. Nothing here is a secret: the metadata
// key is the connection's ownership token and the application id is the platform's public client id.
func (s Service) azureSetup(c domain.Connection) Setup {
	e, err := domain.ParseAzureExportRef(c.CredentialRef)
	if err != nil { // validated on creation; defensive
		return Setup{ExternalID: c.ExternalID, RequiredPermissions: []string{}, PermissionsPolicy: map[string]any{}}
	}
	key := domain.AzureOwnershipKey(c.ExternalID)
	app := s.PlatformAzureAppID
	st := Setup{
		ExternalID:          c.ExternalID,
		PlatformAccountID:   app,
		RequiredPermissions: []string{"Storage Blob Data Reader on the export container (read-only)"},
		PermissionsPolicy: map[string]any{
			"container":          e.Account + "/" + e.Container,
			"container_metadata": map[string]string{"key": key, "value": "1"},
			"role_assignment":    map[string]string{"role": "Storage Blob Data Reader", "assignee": app},
		},
	}
	st.Steps = []string{
		fmt.Sprintf("In subscription %s create a Cost Management export of type \"Cost and usage details (FOCUS)\", version 1.0, "+
			"a daily export of month-to-date costs, format CSV, into storage account %s, container %s, directory %s. "+
			"Data is exported from the day you create it (use \"Export all data\" for history), and the first delivery can take up to 24 hours.",
			c.AccountRef, e.Account, e.Container, e.Dir),
		fmt.Sprintf("Prove you control the container by marking it: az storage container metadata update --account-name %s --name %s --metadata %s=1 --auth-mode login",
			e.Account, e.Container, key),
	}
	if app != "" {
		st.Steps = append(st.Steps,
			fmt.Sprintf("Create the platform application's service principal in your tenant: az ad sp create --id %s", app),
			fmt.Sprintf("Grant it read access to the container only: az role assignment create --assignee %s --role \"Storage Blob Data Reader\" "+
				"--scope \"$(az storage account show --name %s --query id -o tsv)/blobServices/default/containers/%s\"; "+
				"deploy/azure/customer-access.tf does steps 2 to 4 in Terraform.", app, e.Account, e.Container))
	} else {
		st.Steps = append(st.Steps, "Contact support for the application id to grant access to (it is not configured on this platform yet).")
	}
	st.Steps = append(st.Steps, "Verify the connection. Nothing else is shared: the platform reads only the export directory of that container.")
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

// announceError tells the tenant a connection needs attention. One notification per connection and day at most.
func (s Service) announceError(ctx context.Context, tenantID string, c domain.Connection, reason string) {
	if s.Notify == nil {
		return
	}
	_ = s.Notify.Publish(ctx, tenantID, notify.Event{Type: notify.TypeConnectionError, Severity: notify.Warning,
		Title: fmt.Sprintf("%s connection %s needs attention", strings.ToUpper(string(c.Provider)), c.AccountRef), Body: reason, Path: "/cloud",
		DedupeKey: "connection:" + c.ID + ":error:" + time.Now().UTC().Format(time.DateOnly),
		Attrs:     map[string]string{"connection_id": c.ID, "provider": string(c.Provider)}})
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
		s.announceError(ctx, tenantID, c, msg)
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
		if s.pulledRecently(ctx, j) {
			continue
		}
		if err := s.SyncNow(ctx, j.TenantID, j.ProjectID, j.ConnectionID); err != nil && !errors.Is(err, asynq.ErrDuplicateTask) {
			s.Log.Error("enqueue sync", "connection", j.ConnectionID, "err", err)
			continue
		}
		n++
	}
	return n, nil
}

// pulledRecently implements the organization's minimum interval between data pulls. The platform schedule is every
// 6 hours; an organization that asks for a longer interval has the runs in between skipped. At the default (6 hours)
// nothing extra is read.
func (s Service) pulledRecently(ctx context.Context, j domain.JobRef) bool {
	if s.Settings == nil {
		return false
	}
	hours := s.Settings.Int(ctx, j.TenantID, settings.KeySyncMinIntervalH)
	if hours <= 6 {
		return false
	}
	c, err := s.Repo.Get(ctx, j.TenantID, j.ConnectionID)
	if err != nil || c.LastSyncAt == nil {
		return false
	}
	return time.Since(*c.LastSyncAt) < time.Duration(hours)*time.Hour-30*time.Minute // slack for schedule jitter
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
		_ = s.Repo.FinishRun(ctx, tenantID, runID, false, 0, &msg, nil)
		if domain.NeedsCustomerAction(cause) {
			_ = s.Repo.SetStatus(ctx, tenantID, connectionID, domain.StatusError, &msg)
			s.announceError(ctx, tenantID, c, msg)
			observability.CloudSyncTotal.WithLabelValues(label, "denied").Inc()
		} else {
			observability.CloudSyncTotal.WithLabelValues(label, "failed").Inc()
		}
		observability.CloudSyncDuration.WithLabelValues(label).Observe(time.Since(start).Seconds())
		return cause
	}

	from, to := s.window(c)
	var res focus.IngestResult
	quality := domain.Unchecked("nothing to fetch in this window", s.now())
	if from.Before(to) {
		req := domain.UsageRequest{Connection: c, From: from, To: to}
		batches, err := p.GetUsage(ctx, req)
		if err != nil {
			return fail(err)
		}
		if res, err = s.Ingestor.Ingest(ctx, tenantID, c.ProjectID, connectionID, batches); err != nil {
			return fail(fmt.Errorf("ingest: %w", err))
		}
		quality = s.reconcile(ctx, p, req, res)
	}
	through := to.AddDate(0, 0, -1)
	if first, bad := quality.EarliestMismatch(); bad {
		// Self-healing: do not move the cursor past a day that does not add up. The next sync re-reads from there
		// (rewinding at most maxRewindDays, so a day that can never match cannot cause ever larger re-reads).
		if rewind := first.AddDate(0, 0, -1); rewind.Before(through) {
			through = laterOf(rewind, through.AddDate(0, 0, -maxRewindDays))
		}
	}
	if err := s.Repo.MarkSynced(ctx, tenantID, connectionID, through, res.Records, &quality); err != nil {
		return fail(err)
	}
	if err := s.Repo.FinishRun(ctx, tenantID, runID, true, res.Records, nil, &quality); err != nil {
		return err
	}
	observability.CloudSyncTotal.WithLabelValues(label, "succeeded").Inc()
	observability.CloudSyncReconciliation.WithLabelValues(label, quality.Status).Inc()
	observability.CloudSyncDuration.WithLabelValues(label).Observe(time.Since(start).Seconds())
	if quality.Status == domain.QualityMismatch {
		s.announceMismatch(ctx, tenantID, c, quality)
	}

	if res.Records > 0 { // chain: a finished sync queues the carbon calculation for the touched days
		if err := s.Queue.Enqueue(ctx, queue.TaskCalculateCarbon, queue.WindowPayload{
			TenantID: tenantID, ProjectID: c.ProjectID,
			From: res.From.Format(time.DateOnly), To: res.To.AddDate(0, 0, 1).Format(time.DateOnly),
		}); err != nil {
			// The data is stored and the cursor moved. Failing the job would re-read (and re-bill) the whole window
			// just to queue a calculation; the daily recalculation covers the last 35 days anyway.
			s.Log.Warn("carbon calculation not queued after sync; the daily recalculation will pick it up", "connection", connectionID, "err", err)
		}
	}
	return nil
}

// maxRewindDays bounds how far a mismatch can pull the sync cursor back.
const maxRewindDays = 14

func laterOf(a, b time.Time) time.Time {
	if a.After(b) {
		return a
	}
	return b
}

// reconcile compares the stored totals with the cloud's own, when the provider can report them. It never fails a
// sync: an unverifiable sync is stored as "unchecked" with the reason.
func (s Service) reconcile(ctx context.Context, p domain.CloudProvider, req domain.UsageRequest, res focus.IngestResult) domain.Quality {
	cp, ok := p.(domain.ControlProvider)
	if !ok {
		return domain.Unchecked("this data source has no independent total to compare with", s.now())
	}
	control, err := cp.GetControlTotals(ctx, req)
	if err != nil {
		s.Log.Warn("control totals unavailable", "connection", req.Connection.ID, "err", err)
		return domain.Unchecked("the cloud's own totals could not be fetched this time", s.now())
	}
	if control == nil {
		return domain.Unchecked("this connection reads an invoice-level export that is its own source of truth", s.now())
	}
	return domain.Reconcile(control, res.Totals, s.now())
}

// announceMismatch tells the tenant (once a day per connection) that some days do not add up.
func (s Service) announceMismatch(ctx context.Context, tenantID string, c domain.Connection, q domain.Quality) {
	if s.Notify == nil {
		return
	}
	_ = s.Notify.Publish(ctx, tenantID, notify.Event{Type: notify.TypeConnectionDataQuality, Severity: notify.Warning,
		Title: fmt.Sprintf("%s connection %s: cost data does not match the cloud", strings.ToUpper(string(c.Provider)), c.AccountRef),
		Body:  q.Note, Path: "/cloud", DedupeKey: "connection:" + c.ID + ":quality:" + time.Now().UTC().Format(time.DateOnly),
		Attrs: map[string]string{"connection_id": c.ID, "provider": string(c.Provider)}})
}

// safeMessage is stored and shown to users: it never contains provider internals or secrets.
func safeMessage(err error) string {
	switch {
	case errors.Is(err, domain.ErrAccessDenied):
		return "Access denied: check the access you granted (AWS role and its read permissions, the platform's read access to the BigQuery dataset, or the platform application's Storage Blob Data Reader role on the Azure container)."
	case errors.Is(err, domain.ErrBigQueryDisabled):
		return "BigQuery is not enabled in your Google Cloud project. Enable the BigQuery API, turn on Cloud Billing export to BigQuery, then verify again."
	case errors.Is(err, domain.ErrOwnershipNotProven):
		return "Ownership of the billing dataset is not proven: add the connection's label to the dataset (see the setup steps), then verify again."
	case errors.Is(err, domain.ErrSubscriptionNotInExport):
		return "The export has no rows for this subscription: check the subscription id and that the export covers it (subscription, resource group or billing scope), then verify again."
	case errors.Is(err, domain.ErrExportNotFound):
		return "Billing export not found: check that the export is enabled, delivered (the first delivery can take up to 48 hours) and that the location is correct, then verify again."
	default:
		return "Sync failed; it will be retried automatically."
	}
}
