// Package application holds the operator console use-cases. Every call first proves the caller is an operator
// (from the database, never from the token), records the access, then runs.
package application

import (
	"context"
	"net/mail"
	"regexp"
	"strings"
	"time"

	"github.com/quixgit/greenops-reliabilix/backend/internal/operator/domain"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/plans"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/queue"
)

type Service struct {
	Repo     domain.Repository
	System   func() domain.System
	Settings domain.SettingsPort
	// Integrations manages external-service credentials (optional: nil answers 404).
	Integrations domain.IntegrationsPort
	// Sources is the data source catalog with its configuration state (wired in internal/app, which knows the
	// configuration). Provider connection counts are merged in from the database.
	Sources func(ctx context.Context) []domain.DataSource
	// Jobs is queue telemetry (optional: nil answers 404).
	Jobs domain.JobsPort
	// Queue starts background jobs (optional: the backfill is then unavailable).
	Queue queue.Enqueuer
}

// Authorize resolves the operator behind a verified subject. write=true additionally demands an admin operator.
// The attempt is logged before anything is returned, so the access log also shows who looked.
func (s Service) Authorize(ctx context.Context, subject, email, action string, write bool) (domain.Operator, error) {
	role, ok, err := s.Repo.Role(ctx, subject)
	if err != nil {
		return domain.Operator{}, err
	}
	if !ok || !role.Valid() {
		return domain.Operator{}, domain.ErrNotOperator
	}
	if write && !role.CanWrite() {
		return domain.Operator{}, domain.ErrReadOnly
	}
	if err := s.Repo.LogAccess(ctx, subject, action); err != nil {
		return domain.Operator{}, err // no access without a record
	}
	return domain.Operator{Subject: subject, Email: email, Role: role}, nil
}

func (s Service) Tenants(ctx context.Context) ([]domain.Tenant, error) {
	return s.Repo.Tenants(ctx, 500)
}

func (s Service) Deliveries(ctx context.Context, status string, limit int) ([]domain.Delivery, error) {
	if status != "" && !domain.DeliveryStatuses[status] {
		return nil, domain.ErrInvalid
	}
	return s.Repo.Deliveries(ctx, status, limit)
}

func (s Service) DeliveryStats(ctx context.Context, hours int) ([]domain.DeliveryStat, error) {
	if hours < 1 || hours > 24*30 {
		return nil, domain.ErrInvalid
	}
	return s.Repo.DeliveryStats(ctx, hours)
}

func (s Service) UnhealthyConnections(ctx context.Context) ([]domain.Connection, error) {
	return s.Repo.UnhealthyConnections(ctx, 200)
}

var serviceRe = regexp.MustCompile(`^[a-z][a-z0-9_-]{1,31}$`)

// Errors lists error groups. status "" means all; service "" means every service.
func (s Service) Errors(ctx context.Context, status, service string, limit int) ([]domain.ErrorGroup, error) {
	if status != "" && !domain.ErrorStatuses[status] {
		return nil, domain.ErrInvalid
	}
	if service != "" && !serviceRe.MatchString(service) {
		return nil, domain.ErrInvalid
	}
	return s.Repo.Errors(ctx, status, service, limit)
}

// SetErrorStatus marks an error group open, resolved or ignored.
func (s Service) SetErrorStatus(ctx context.Context, op domain.Operator, id, status string) error {
	if !domain.ErrorStatuses[status] {
		return domain.ErrInvalid
	}
	done, err := s.Repo.SetErrorStatus(ctx, op.Subject, id, status)
	if err == nil && !done {
		return domain.ErrNotFound
	}
	return err
}

// RetryDelivery puts a failed delivery back in the queue (the sweep picks it up within about a minute).
func (s Service) RetryDelivery(ctx context.Context, op domain.Operator, tenantID, deliveryID string) error {
	done, err := s.Repo.RetryDelivery(ctx, op.Subject, tenantID, deliveryID)
	if err == nil && !done {
		return domain.ErrNotFound
	}
	return err
}

func (s Service) SetDestinationEnabled(ctx context.Context, op domain.Operator, tenantID, destinationID string, enabled bool) error {
	done, err := s.Repo.SetDestinationEnabled(ctx, op.Subject, tenantID, destinationID, enabled)
	if err == nil && !done {
		return domain.ErrNotFound
	}
	return err
}

func (s Service) SystemInfo() domain.System { return s.System() }

// Audit lists the newest lines of the operators' own access log.
func (s Service) Audit(ctx context.Context, limit int) ([]domain.AuditEntry, error) {
	if limit < 1 || limit > 500 {
		return nil, domain.ErrInvalid
	}
	return s.Repo.Audit(ctx, limit)
}

// providerOf maps a catalog source to the provider name stored on customer connections.
var providerOf = map[string]string{"aws-cost": "aws", "gcp-billing": "gcp"}

// DataSources returns the registry. A connector source gets the number of real connections; a source with failing
// connections is marked "attention". Nothing is invented: a source nobody connected shows zero.
func (s Service) DataSources(ctx context.Context) ([]domain.DataSource, error) {
	if s.Sources == nil {
		return []domain.DataSource{}, nil
	}
	usage, err := s.Repo.ProviderUsage(ctx)
	if err != nil {
		return nil, err
	}
	by := map[string]domain.ProviderUsage{}
	for _, u := range usage {
		by[u.Provider] = u
	}
	out := s.Sources(ctx)
	for i := range out {
		p, ok := providerOf[out[i].ID]
		if !ok {
			continue
		}
		u := by[p]
		out[i].Connections, out[i].Organizations, out[i].InError, out[i].LastSyncAt = &u.Connections, &u.Organizations, &u.InError, u.LastSyncAt
		switch {
		case out[i].State == "planned":
		case u.InError > 0:
			out[i].State = "attention"
		case u.Connections > 0:
			out[i].State = "ready"
		default:
			out[i].State = "not_configured"
		}
	}
	return out, nil
}

// Operators lists who is an operator and how active each one is.
func (s Service) Operators(ctx context.Context) ([]domain.OperatorRecord, error) {
	return s.Repo.Operators(ctx)
}

// PlanRow is a catalog plan with how many organizations are on it.
type PlanRow struct {
	plans.Plan
	Organizations int `json:"organizations"`
}

// Plans returns the catalog with the number of organizations per plan. Organizations on a plan id that is not in the
// catalog are reported under their own id so nothing is hidden.
func (s Service) Plans(ctx context.Context) ([]PlanRow, error) {
	tenants, err := s.Repo.Tenants(ctx, 500)
	if err != nil {
		return nil, err
	}
	count := map[string]int{}
	for _, t := range tenants {
		count[t.Plan]++
	}
	var out []PlanRow
	for _, p := range plans.Catalog() {
		out = append(out, PlanRow{Plan: p, Organizations: count[p.ID]})
		delete(count, p.ID)
	}
	for id, n := range count {
		out = append(out, PlanRow{Plan: plans.Plan{ID: id, Name: id, Description: "Not in the catalog."}, Organizations: n})
	}
	return out, nil
}

// PlanChange is what an operator sets for an organization: the plan and the state of its subscription.
type PlanChange struct {
	Plan      string
	Status    string // active (default) | trialing | past_due | canceled
	TrialEnd  *time.Time
	PeriodEnd *time.Time
	Reason    string
}

// AssignPlan writes the plan's limits and module switches for one organization (locked operator values, in the
// settings history) and records the plan, its status, dates and the reason in the subscription and in the organization's
// audit log. The settings are written first: if the second step fails the limits are already right and a repeat
// completes the label.
func (s Service) AssignPlan(ctx context.Context, op domain.Operator, tenantID string, c PlanChange) error {
	p, ok := plans.Lookup(c.Plan)
	c.Reason = strings.TrimSpace(c.Reason)
	if c.Status == "" {
		c.Status = "active"
	}
	switch c.Status {
	case "active", "trialing", "past_due", "canceled":
	default:
		return domain.ErrInvalid
	}
	if !ok || len(c.Reason) < 3 || len(c.Reason) > 300 || s.Settings == nil || (c.Status == "trialing" && c.TrialEnd == nil) {
		return domain.ErrInvalid
	}
	if err := s.Settings.SetTenantOverride(ctx, op.Subject, tenantID, p.Values()); err != nil {
		return err
	}
	done, err := s.Repo.SetSubscription(ctx, op.Subject, tenantID, c.Plan, c.Status, c.TrialEnd, c.PeriodEnd, c.Reason)
	if err == nil && !done {
		return domain.ErrNotFound
	}
	return err
}

// Subscription is the state of an organization's subscription.
func (s Service) Subscription(ctx context.Context, tenantID string) (domain.Subscription, error) {
	return s.Repo.Subscription(ctx, tenantID)
}

// BillingEvents lists the latest payment notifications and invoice requests.
func (s Service) BillingEvents(ctx context.Context) ([]domain.BillingEvent, error) {
	return s.Repo.BillingEvents(ctx, 100)
}

// AssignOperator makes a person who has signed in an operator, or changes their role. The e-mail identifies the person.
func (s Service) AssignOperator(ctx context.Context, op domain.Operator, email string, role domain.Role) error {
	addr, err := mail.ParseAddress(strings.TrimSpace(email))
	if err != nil || !role.Valid() || len(addr.Address) > 320 {
		return domain.ErrInvalid
	}
	_, err = s.Repo.AssignOperator(ctx, op.Subject, addr.Address, role)
	return err
}

// WithdrawOperator removes operator access. The database refuses to remove the caller or the last admin.
func (s Service) WithdrawOperator(ctx context.Context, op domain.Operator, subject string) error {
	if subject == "" || len(subject) > 255 {
		return domain.ErrInvalid
	}
	done, err := s.Repo.WithdrawOperator(ctx, op.Subject, subject)
	if err == nil && !done {
		return domain.ErrNotFound
	}
	return err
}

// Backfill limits mirror the carbon module's: the provider throttles and bills by request.
const (
	backfillMaxDays     = 400
	backfillMaxRequests = 2000
	backfillChunkDays   = 9
)

// BackfillPlan is what a grid-history backfill would do.
type BackfillPlan struct {
	Regions          []string `json:"regions"`
	Days             int      `json:"days"`
	EstimatedRequest int      `json:"estimated_requests"`
	Queued           bool     `json:"queued"`
}

// BackfillGridHistory fetches past grid readings for the regions in use. dryRun only reports the plan (regions, days and
// the number of provider requests), so cost is seen before anything is asked of the provider.
func (s Service) BackfillGridHistory(ctx context.Context, from, to time.Time, dryRun bool) (BackfillPlan, error) {
	days := int(to.Sub(from).Hours() / 24)
	if days < 1 || days > backfillMaxDays || to.After(time.Now().UTC().Add(24*time.Hour)) {
		return BackfillPlan{}, domain.ErrInvalid
	}
	regions, err := s.Repo.CarbonRegions(ctx)
	if err != nil {
		return BackfillPlan{}, err
	}
	p := BackfillPlan{Regions: regions, Days: days, EstimatedRequest: len(regions) * ((days + backfillChunkDays - 1) / backfillChunkDays)}
	if p.EstimatedRequest > backfillMaxRequests {
		return p, domain.ErrInvalid
	}
	if dryRun || len(regions) == 0 {
		return p, nil
	}
	if s.Queue == nil {
		return p, domain.ErrNotFound
	}
	err = s.Queue.Enqueue(ctx, queue.TaskBackfillIntensity, queue.BackfillPayload{Regions: regions, From: from.Format(time.DateOnly), To: to.Format(time.DateOnly)})
	p.Queued = err == nil
	return p, err
}
