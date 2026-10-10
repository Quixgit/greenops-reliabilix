// Package application holds the operator console use-cases. Every call first proves the caller is an operator
// (from the database, never from the token), records the access, then runs.
package application

import (
	"context"
	"encoding/json"
	"net/mail"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/quixgit/greenops-reliabilix/backend/internal/operator/domain"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/plans"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/queue"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/settings"
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
var providerOf = map[string]string{"aws-cost": "aws", "gcp-billing": "gcp", "azure-billing": "azure"}

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

// RegionSources lists every region in use with what each grid source has for it, the operator's override (if any) and
// the source the current policy would use. Priority and overrides come from the platform settings.
func (s Service) RegionSources(ctx context.Context) ([]domain.RegionSources, error) {
	rows, err := s.Repo.CarbonRegionSources(ctx)
	if err != nil {
		return nil, err
	}
	priority, overrides := []string{"electricitymaps", "uk_carbon_intensity", "eia", "energy_charts", "ember"}, map[string]string{}
	if s.Settings != nil {
		if items, err := s.Settings.PlatformItems(ctx); err == nil {
			for _, it := range items {
				if it.Def == nil {
					continue
				}
				var list []string
				switch it.Key {
				case settings.KeyCarbonPriority:
					if json.Unmarshal(it.Value, &list) == nil && len(list) > 0 {
						priority = list
					}
				case settings.KeyCarbonOverrides:
					if json.Unmarshal(it.Value, &list) == nil {
						for _, e := range list {
							if region, src, ok := strings.Cut(e, "="); ok {
								overrides[region] = src
							}
						}
					}
				}
			}
		}
	}
	// Regions in use with no data at all still appear: the table shows what is missing.
	byRegion := map[string]*domain.RegionSources{}
	var order []string
	add := func(region string) *domain.RegionSources {
		if r, ok := byRegion[region]; ok {
			return r
		}
		r := &domain.RegionSources{Region: region, Override: overrides[region], Sources: []domain.RegionSourceRow{}}
		byRegion[region] = r
		order = append(order, region)
		return r
	}
	for _, row := range rows {
		r := add(row.Region)
		r.Sources = append(r.Sources, row)
	}
	regions, err := s.Repo.CarbonRegions(ctx)
	if err != nil {
		return nil, err
	}
	for _, region := range regions {
		add(region)
	}
	out := make([]domain.RegionSources, 0, len(order))
	for _, region := range order {
		r := byRegion[region]
		have := map[string]domain.RegionSourceRow{}
		for _, x := range r.Sources {
			have[x.Provider] = x
		}
		seq := []string{}
		if r.Override != "" {
			seq = append(seq, r.Override)
		}
		seq = append(seq, priority...)
		for _, p := range seq { // hourly data first, then monthly figures, like the resolver
			if x, ok := have[p]; ok && x.Hourly > 0 {
				r.Effective = p
				break
			}
		}
		if r.Effective == "" {
			for _, p := range seq {
				if x, ok := have[p]; ok && x.Months > 0 {
					r.Effective = p
					break
				}
			}
		}
		out = append(out, *r)
	}
	return out, nil
}

// UnitCosts are the platform's unit costs in USD (settings group "costs"); a missing value counts as zero.
type UnitCosts struct {
	Base, UsageRowsPerMillion, CalcRowsPerThousand, SyncRun, DeliveriesPerThousand, Report, K8sSamplesPerMillion float64
}

// Cost is the estimated cost of serving an organization for the window the usage was counted over.
func (c UnitCosts) Cost(u domain.OrgUsage) float64 {
	return c.Base + float64(u.UsageRows)/1e6*c.UsageRowsPerMillion + float64(u.CalculationRows)/1e3*c.CalcRowsPerThousand +
		float64(u.SyncRuns)*c.SyncRun + float64(u.Deliveries)/1e3*c.DeliveriesPerThousand + float64(u.Reports)*c.Report +
		float64(u.K8sSamples)/1e6*c.K8sSamplesPerMillion
}

func (s Service) unitCosts(ctx context.Context) UnitCosts {
	var c UnitCosts
	if s.Settings == nil {
		return c
	}
	items, err := s.Settings.PlatformItems(ctx)
	if err != nil {
		return c
	}
	for _, it := range items {
		if it.Def == nil {
			continue
		}
		var v float64
		if json.Unmarshal(it.Value, &v) != nil {
			continue
		}
		switch it.Key {
		case settings.KeyCostBase:
			c.Base = v
		case settings.KeyCostUsageRows:
			c.UsageRowsPerMillion = v
		case settings.KeyCostCalcRows:
			c.CalcRowsPerThousand = v
		case settings.KeyCostSyncRun:
			c.SyncRun = v
		case settings.KeyCostDeliveries:
			c.DeliveriesPerThousand = v
		case settings.KeyCostReport:
			c.Report = v
		case settings.KeyCostK8sSamples:
			c.K8sSamplesPerMillion = v
		}
	}
	return c
}

// OrgUsage lists what each organization used over the last `days` days with the estimated cost of serving it, the most
// expensive first. The estimate is the counts times unit costs from settings: an estimate, not an accounting figure.
func (s Service) OrgUsage(ctx context.Context, days int) ([]domain.OrgUsage, UnitCosts, error) {
	if days < 1 || days > 366 {
		return nil, UnitCosts{}, domain.ErrInvalid
	}
	rows, err := s.Repo.OrgUsage(ctx, days)
	if err != nil {
		return nil, UnitCosts{}, err
	}
	c := s.unitCosts(ctx)
	for i := range rows {
		rows[i].EstimatedCost = c.Cost(rows[i])
	}
	sort.Slice(rows, func(a, b int) bool { return rows[a].EstimatedCost > rows[b].EstimatedCost })
	return rows, c, nil
}

// RecalculateAll queues the recalculation of every project over the last `days` days (the stored carbon then uses the grid
// data that exists now). It is idempotent: a window is replaced as a whole.
func (s Service) RecalculateAll(ctx context.Context, days int) (from, to time.Time, err error) {
	if days < 1 || days > 400 || s.Queue == nil {
		return from, to, domain.ErrInvalid
	}
	to = time.Now().UTC().Truncate(24*time.Hour).AddDate(0, 0, 1)
	from = to.AddDate(0, 0, -days)
	err = s.Queue.Enqueue(ctx, queue.TaskRecalculateWindow, queue.RecalcPayload{From: from.Format(time.DateOnly), To: to.Format(time.DateOnly)})
	return from, to, err
}
