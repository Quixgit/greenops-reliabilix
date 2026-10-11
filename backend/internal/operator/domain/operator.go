// Package domain defines the platform operator console: who may use it and the cross-tenant metadata it shows.
// Everything here is metadata about the platform's own operation (counts, statuses, error texts); the console never
// carries tenant payloads, destination addresses, tokens or usage data.
package domain

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/integrations"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/queue"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/settings"
)

type Role string

const (
	Viewer Role = "viewer" // read only
	Admin  Role = "admin"  // may also retry deliveries and disable destinations
)

func (r Role) Valid() bool    { return r == Viewer || r == Admin }
func (r Role) CanWrite() bool { return r == Admin }

var (
	ErrNotOperator = errors.New("not a platform operator")
	ErrReadOnly    = errors.New("operator role is read-only")
	ErrNotFound    = errors.New("not found")
	ErrInvalid     = errors.New("invalid request")
	// ErrRule is a refused change that the caller can understand and fix (the wrapped message is safe to show).
	ErrRule = errors.New("not allowed")
)

type Operator struct {
	Subject string `json:"subject"`
	Email   string `json:"email"`
	Role    Role   `json:"role"`
}

type Tenant struct {
	ID                 string    `json:"id"`
	Name               string    `json:"name"`
	Plan               string    `json:"plan"`
	CreatedAt          time.Time `json:"created_at"`
	Members            int       `json:"members"`
	Projects           int       `json:"projects"`
	Connections        int       `json:"connections"`
	ConnectionsInError int       `json:"connections_in_error"`
	Destinations       int       `json:"destinations"`
	FailedDeliveries7d int       `json:"failed_deliveries_7d"`
}

type Delivery struct {
	TenantID        string     `json:"tenant_id"`
	TenantName      string     `json:"tenant_name"`
	ID              string     `json:"id"`
	DestinationID   *string    `json:"destination_id"`
	Kind            string     `json:"kind"`
	DestinationName string     `json:"destination_name"`
	EventType       string     `json:"event_type"`
	Severity        string     `json:"severity"`
	Status          string     `json:"status"`
	Attempts        int        `json:"attempts"`
	NextAttemptAt   time.Time  `json:"next_attempt_at"`
	LastError       *string    `json:"last_error"`
	CreatedAt       time.Time  `json:"created_at"`
	SentAt          *time.Time `json:"sent_at"`
}

type DeliveryStat struct {
	Kind   string `json:"kind"`
	Status string `json:"status"`
	Count  int64  `json:"count"`
}

type Connection struct {
	TenantID   string     `json:"tenant_id"`
	TenantName string     `json:"tenant_name"`
	Provider   string     `json:"provider"`
	AccountRef string     `json:"account_ref"`
	SyncStatus string     `json:"sync_status"`
	LastError  *string    `json:"last_error"`
	LastSyncAt *time.Time `json:"last_sync_at"`
}

// Integration describes one external dependency. Status "ready" means a real credential is configured,
// "stub" means a development stand-in is running, "missing" means the feature is unavailable until it is set up.
// ErrorGroup is one kind of error of a platform service, with how often and when it happened (ADR-0021).
type ErrorGroup struct {
	ID          string     `json:"id"`
	Service     string     `json:"service"`
	Component   string     `json:"component"`
	Severity    string     `json:"severity"`
	Message     string     `json:"message"`
	Status      string     `json:"status"`
	Occurrences int64      `json:"occurrences"`
	FirstSeen   time.Time  `json:"first_seen"`
	LastSeen    time.Time  `json:"last_seen"`
	TenantID    *string    `json:"tenant_id"`
	TenantName  *string    `json:"tenant_name"`
	RequestID   *string    `json:"last_request_id"`
	HandledBy   *string    `json:"handled_by"`
	HandledAt   *time.Time `json:"handled_at"`
}

var ErrorStatuses = map[string]bool{"open": true, "resolved": true, "ignored": true}

type Integration struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	Note   string `json:"note"`
}

type System struct {
	Version           string        `json:"version"`
	Env               string        `json:"env"`
	ExpectedMigration int           `json:"expected_migration"`
	Integrations      []Integration `json:"integrations"`
	// Runtime and Database describe the process that answered and its connection to the database; they are filled in
	// by the handler at the moment of the request, never stored.
	Runtime  *Runtime       `json:"runtime,omitempty"`
	Database *DatabaseStats `json:"database,omitempty"`
}

// Runtime is the answering process: since when it runs and how much it holds.
type Runtime struct {
	StartedAt  time.Time `json:"started_at"`
	GoVersion  string    `json:"go_version"`
	Goroutines int       `json:"goroutines"`
	HeapMB     float64   `json:"heap_mb"`
	CPUs       int       `json:"cpus"`
}

// DatabaseStats is the connection pool of the API and the time one round trip takes.
type DatabaseStats struct {
	OK          bool    `json:"ok"`
	PingMS      float64 `json:"ping_ms"`
	Connections int32   `json:"connections"`
	InUse       int32   `json:"in_use"`
	Max         int32   `json:"max"`
}

// AuditEntry is one line of the operators' own access log: who looked at or changed what, and when.
type AuditEntry struct {
	ID            int64     `json:"id"`
	At            time.Time `json:"at"`
	OperatorSub   string    `json:"operator_sub"`
	OperatorEmail string    `json:"operator_email"`
	Role          string    `json:"role"`
	Action        string    `json:"action"`
}

// ProviderUsage is how many customer connections use one cloud provider (counts only).
type ProviderUsage struct {
	Provider      string     `json:"provider"`
	Connections   int        `json:"connections"`
	Organizations int        `json:"organizations"`
	InError       int        `json:"in_error"`
	LastSyncAt    *time.Time `json:"last_sync_at"`
}

// Access says who pays for and who supplies a data source.
type Access string

const (
	AccessFree     Access = "free"
	AccessPaid     Access = "paid"
	AccessCustomer Access = "customer_provided"
	AccessInternal Access = "internal"
)

// DataSource is one entry of the source registry: what the platform reads, where from, under what terms, and in what
// state. State comes from the real configuration; counts only from real connections. "planned" is a source that is
// listed so it is not forgotten, and never reports a number.
type DataSource struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Category string `json:"category"` // grid_intensity | billing | usage | kubernetes | methodology
	Access   Access `json:"access"`
	// State: ready (credentials or connections in place), attention (connections with errors), not_configured, planned.
	State    string   `json:"state"`
	Metrics  []string `json:"metrics"`
	Coverage string   `json:"coverage"`
	Terms    string   `json:"terms"`
	Note     string   `json:"note"`
	// IntegrationID is the entry of the integrations catalog that holds this source's keys and addresses ("" when the
	// source has no platform-level settings, e.g. customer-provided data). The settings are edited on the source's page.
	IntegrationID string `json:"integration_id"`
	// Present only for connector sources that customers connect.
	Connections   *int       `json:"connections"`
	Organizations *int       `json:"organizations"`
	InError       *int       `json:"in_error"`
	LastSyncAt    *time.Time `json:"last_sync_at"`
}

var DeliveryStatuses = map[string]bool{"pending": true, "sent": true, "failed": true, "skipped": true}

// Repository reads across tenants through narrow SECURITY DEFINER functions.
type Repository interface {
	Role(ctx context.Context, subject string) (Role, bool, error)
	LogAccess(ctx context.Context, subject, action string) error
	Tenants(ctx context.Context, limit int) ([]Tenant, error)
	Deliveries(ctx context.Context, status string, limit int) ([]Delivery, error)
	DeliveryStats(ctx context.Context, hours int) ([]DeliveryStat, error)
	UnhealthyConnections(ctx context.Context, limit int) ([]Connection, error)
	Audit(ctx context.Context, limit int) ([]AuditEntry, error)
	ProviderUsage(ctx context.Context) ([]ProviderUsage, error)
	Operators(ctx context.Context) ([]OperatorRecord, error)
	SetSubscription(ctx context.Context, actor, tenantID, plan, status string, trialEnd, periodEnd *time.Time, reason string) (bool, error)
	Subscription(ctx context.Context, tenantID string) (Subscription, error)
	BillingEvents(ctx context.Context, limit int) ([]BillingEvent, error)
	APIUsage(ctx context.Context, days int) ([]APIUsage, error)
	CarbonRegionSources(ctx context.Context) ([]RegionSourceRow, error)
	OrgUsage(ctx context.Context, days int) ([]OrgUsage, error)
	// IntensityProvenance counts calculation rows by the source and basis of their intensity over the last days.
	IntensityProvenance(ctx context.Context, days int) ([]ProvenanceRow, error)
	// SourceDivergence is, per region and source, the median hourly reading of the last days and the newest monthly figure.
	SourceDivergence(ctx context.Context, days int) ([]DivergenceRow, error)
	AssignOperator(ctx context.Context, actor, email string, role Role) (string, error)
	WithdrawOperator(ctx context.Context, actor, subject string) (bool, error)
	// CarbonRegions lists the regions that have carbon calculations for any organization.
	CarbonRegions(ctx context.Context) ([]string, error)
	Errors(ctx context.Context, status, service string, limit int) ([]ErrorGroup, error)
	SetErrorStatus(ctx context.Context, actor, id, status string) (bool, error)
	RetryDelivery(ctx context.Context, actor, tenantID, deliveryID string) (bool, error)
	SetDestinationEnabled(ctx context.Context, actor, tenantID, destinationID string, enabled bool) (bool, error)
}

// IntegrationsPort is what the console needs to manage external-service credentials (ADR-0029), wired in
// internal/app. Values of secret fields never come back through it.
type IntegrationsPort interface {
	Statuses(ctx context.Context) ([]integrations.Status, error)
	Set(ctx context.Context, actor, integration string, values map[string]*string) error
	// Test checks the integration with its current values and returns a short, safe message.
	Test(ctx context.Context, integration string) (string, error)
}

// JobsPort is read-only queue telemetry (Redis/Asynq). It carries no payloads.
type JobsPort interface {
	Queues() ([]queue.QueueStat, error)
	Failed(limit int) ([]queue.FailedTask, error)
	Schedules() ([]queue.Schedule, error)
	// Retry runs a failed task again now; Delete removes it. Both only touch tasks that failed (retry or archived).
	Retry(queue, id string) error
	Delete(queue, id string) error
}

// OrgUsage is what an organization used in a window (counts) and the estimated cost of serving it.
type OrgUsage struct {
	TenantID        string  `json:"tenant_id"`
	Name            string  `json:"name"`
	Plan            string  `json:"plan"`
	UsageRows       int64   `json:"usage_rows"`
	CalculationRows int64   `json:"calculation_rows"`
	SyncRuns        int64   `json:"sync_runs"`
	Deliveries      int64   `json:"deliveries"`
	Reports         int64   `json:"reports"`
	K8sSamples      int64   `json:"k8s_samples"`
	Members         int     `json:"members"`
	Connections     int     `json:"connections"`
	EstimatedCost   float64 `json:"estimated_cost_usd"`
}

// RegionSourceRow is what one source has for one region in use.
type RegionSourceRow struct {
	Region   string     `json:"-"`
	Provider string     `json:"provider"`
	Hourly   int        `json:"hourly_readings_30d"`
	Months   int        `json:"monthly_figures"`
	Newest   *time.Time `json:"newest"`
}

// RegionSources is a region in use with the data each source has and the source the policy picks.
type RegionSources struct {
	Region    string            `json:"region"`
	Override  string            `json:"override"`  // "" = automatic
	Effective string            `json:"effective"` // the source that would be used now; "" = no data at all
	Sources   []RegionSourceRow `json:"sources"`
}

// APIUsage is one provider's request count for a day.
type APIUsage struct {
	Provider string    `json:"provider"`
	Day      time.Time `json:"day"`
	Requests int       `json:"requests"`
	Errors   int       `json:"errors"`
}

// Subscription is an organization's plan state.
type Subscription struct {
	Plan        string     `json:"plan"`
	Status      string     `json:"status"` // none when no subscription is recorded
	Source      string     `json:"source"`
	TrialEndsAt *time.Time `json:"trial_ends_at"`
	PeriodEnd   *time.Time `json:"period_end"`
	UpdatedBy   string     `json:"updated_by"`
	UpdatedAt   *time.Time `json:"updated_at"`
}

// BillingEvent is a payment provider notification that was applied, or a request for an invoice.
type BillingEvent struct {
	At               time.Time `json:"at"`
	Provider         string    `json:"provider"`
	Kind             string    `json:"kind"`
	OrganizationID   *string   `json:"organization_id"`
	OrganizationName string    `json:"organization_name"`
	Plan             string    `json:"plan"`
	Detail           string    `json:"detail"`
}

// OperatorRecord is one platform operator (identity is a database fact, ADR-0020).
type OperatorRecord struct {
	Subject    string     `json:"subject"`
	Email      string     `json:"email"`
	Role       Role       `json:"role"`
	CreatedAt  time.Time  `json:"created_at"`
	LastSeenAt *time.Time `json:"last_seen_at"`
	Reads30d   int        `json:"actions_30d"`
}

// SettingsPort is what the console needs from the settings engine (wired in internal/app).
type SettingsPort interface {
	Groups() []settings.Group
	PlatformItems(ctx context.Context) ([]settings.Item, error)
	SetPlatform(ctx context.Context, actor string, values map[string]json.RawMessage) error
	ResetPlatform(ctx context.Context, actor string, keys []string) error
	OperatorTenantItems(ctx context.Context, tenantID string) ([]settings.Item, error)
	SetTenantOverride(ctx context.Context, actor, tenantID string, values map[string]json.RawMessage) error
	ClearTenantOverride(ctx context.Context, actor, tenantID string, keys []string) error
	OperatorHistory(ctx context.Context, tenantID *string, limit int) ([]settings.HistoryEntry, error)
}

// ProvenanceRow is how many calculation rows took their intensity from a source on a basis. An empty source is a row
// calculated before the origin was recorded.
type ProvenanceRow struct {
	Source string `json:"source"`
	Basis  string `json:"basis"`
	Rows   int64  `json:"rows"`
}

// AggregationPolicy is the source policy in force, as the platform settings hold it.
type AggregationPolicy struct {
	Priority          []string          `json:"priority"`
	Overrides         map[string]string `json:"overrides"`
	MinDayReadings    int               `json:"min_day_readings"`
	MaxAgeHours       int               `json:"max_age_hours"`
	SourceMaxAgeHours map[string]int    `json:"source_max_age_hours"`
	DailyRequestCap   int64             `json:"daily_request_cap"`
}

// AggregationSource is one grid-intensity source: what kind of figure it gives, its place in the policy and how fresh
// its data is.
type AggregationSource struct {
	ID          string     `json:"id"`
	Label       string     `json:"label"`
	Measure     string     `json:"measure"`     // average: the only measure that feeds carbon figures
	Granularity string     `json:"granularity"` // hourly | monthly
	Computed    bool       `json:"computed"`    // derived from generation by fuel, not a published intensity
	Licence     string     `json:"licence"`
	Position    int        `json:"position"` // 1-based place in the priority list; 0: not in the policy, never used
	Pinned      int        `json:"pinned_regions"`
	Regions     int        `json:"regions"`
	Hourly30d   int        `json:"hourly_readings_30d"`
	Months      int        `json:"monthly_figures"`
	Newest      *time.Time `json:"newest"`
	MaxAgeHours int        `json:"max_age_hours"`
	Freshness   string     `json:"freshness"` // fresh | aging | stale | none
}

// Provenance is where the figures of the last days took their intensity from.
type Provenance struct {
	Days         int             `json:"days"`
	Total        int64           `json:"total"`
	Unattributed int64           `json:"unattributed"`
	PreciseShare float64         `json:"precise_share"` // share of attributed rows that are a mean of that day's own readings
	Rows         []ProvenanceRow `json:"rows"`
}

// AggregationRule is a rule the engine applies when it combines sources, stated so it can be read and checked.
type AggregationRule struct {
	Code  string `json:"code"`
	Title string `json:"title"`
	Text  string `json:"text"`
}

// DiagStatus is the outcome of one self-test of the platform.
type DiagStatus string

const (
	DiagOK      DiagStatus = "ok"
	DiagWarn    DiagStatus = "warn"
	DiagFail    DiagStatus = "fail"
	DiagSkipped DiagStatus = "skipped" // nothing to test here (not configured), which is not a problem
)

// DiagCheck is one self-test: what it looks at and how to run it. Run returns a short message that is safe to show an operator
// (never a secret, a payload or a tenant's data).
type DiagCheck struct {
	ID          string
	Group       string
	Name        string
	Description string
	Run         func(ctx context.Context) (DiagStatus, string)
}

// DiagResult is the answer of one run of a self-test.
type DiagResult struct {
	ID          string     `json:"id"`
	Group       string     `json:"group"`
	Name        string     `json:"name"`
	Description string     `json:"description"`
	Status      DiagStatus `json:"status"`
	Ms          int64      `json:"ms"`
	Message     string     `json:"message"`
	At          time.Time  `json:"at"`
}

// DivergenceRow is one source's figure for one region as the repository returns it.
type DivergenceRow struct {
	Region   string
	Provider string
	G        float64
	N        int
	Newest   time.Time
	Kind     string // hourly | monthly | annual
}

// DivergenceSource is one source's typical figure for a region (the median of its hourly readings of the last days).
type DivergenceSource struct {
	ID       string     `json:"id"`
	Label    string     `json:"label"`
	G        float64    `json:"g_per_kwh"`
	Readings int        `json:"readings"`
	Newest   *time.Time `json:"newest"`
	Computed bool       `json:"computed"`
	Fresh    bool       `json:"fresh"`
	// VsAnchorPct is how far this figure is from the monthly national figure, in percent; nil without an anchor.
	VsAnchorPct *float64 `json:"vs_anchor_pct"`
}

// DivergenceAnchor is the independent monthly national figure (Ember) a region's hourly sources are held against.
type DivergenceAnchor struct {
	Source string    `json:"source"`
	Label  string    `json:"label"`
	Month  time.Time `json:"month"`
	G      float64   `json:"g_per_kwh"`
}

// Divergence is where the sources of one region disagree. Level: ok (within 1.5x), differs (1.5x to 2.5x) or disagrees (2.5x or
// more, or a lone hourly source twice as far from the national figure). Used is the source the policy takes.
type Divergence struct {
	Region  string             `json:"region"`
	InUse   bool               `json:"in_use"`
	Sources []DivergenceSource `json:"sources"`
	Anchor  *DivergenceAnchor  `json:"anchor"`
	// Inventory is the intensity implied by the national emissions inventory and generation of the newest year (direct emissions).
	Inventory *DivergenceAnchor `json:"inventory"`
	Ratio     float64           `json:"ratio"`
	Level     string            `json:"level"`
	Used      string            `json:"used"`
}

// Aggregation is the whole picture for the console page.
type Aggregation struct {
	Policy     AggregationPolicy   `json:"policy"`
	Sources    []AggregationSource `json:"sources"`
	Provenance Provenance          `json:"provenance"`
	Rules      []AggregationRule   `json:"rules"`
	Regions    int                 `json:"regions_in_use"`
	// Divergence lists the regions where sources can be compared, the most disagreeing first.
	Divergence []Divergence `json:"divergence"`
}
