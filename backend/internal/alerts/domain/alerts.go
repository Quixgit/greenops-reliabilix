// Package domain holds the alert rules: which conditions raise an alert, how they are deduplicated and which
// status changes people may make (ADR-0028). It has no database, HTTP or queue code.
package domain

import (
	"context"
	"errors"
	"fmt"
	"time"
)

type Kind string

const (
	KindBudget          Kind = "budget"
	KindCarbon          Kind = "carbon"
	KindConnectionError Kind = "connection_error"
	KindDataQuality     Kind = "data_quality"
)

type Severity string

const (
	Info     Severity = "info"
	Warning  Severity = "warning"
	Critical Severity = "critical"
)

type Status string

const (
	New          Status = "new"
	Acknowledged Status = "acknowledged"
	Muted        Status = "muted"
	Resolved     Status = "resolved"
)

var (
	ErrNotFound          = errors.New("not found")
	ErrInvalidTransition = errors.New("invalid status transition")
)

// Alert is one stored alert.
type Alert struct {
	ID             string     `json:"id"`
	Kind           Kind       `json:"kind"`
	Severity       Severity   `json:"severity"`
	Status         Status     `json:"status"`
	Title          string     `json:"title"`
	Detail         string     `json:"detail"`
	ProjectID      *string    `json:"project_id"`
	ObjectRef      string     `json:"object_ref"`
	Value          *float64   `json:"value"`
	Threshold      *float64   `json:"threshold"`
	FirstSeenAt    time.Time  `json:"first_seen_at"`
	LastSeenAt     time.Time  `json:"last_seen_at"`
	AcknowledgedBy *string    `json:"acknowledged_by"`
	AcknowledgedAt *time.Time `json:"acknowledged_at"`
	ResolvedAt     *time.Time `json:"resolved_at"`
	DedupeKey      string     `json:"-"`
}

// Condition is what the evaluation observes right now. One condition has one DedupeKey, and so at most one live alert.
type Condition struct {
	DedupeKey string
	Kind      Kind
	Severity  Severity
	Title     string
	Detail    string
	ProjectID *string
	ObjectRef string
	Value     *float64
	Threshold *float64
}

// Default and fixed thresholds. The warning level is an organization setting (alerts.warn_pct); "critical" is always
// the limit itself.
const (
	DefaultWarnPct    = 80.0
	BudgetCriticalPct = 100.0
)

// Thresholds are the organization's alert settings. Zero values mean "default" (warning) and "off" (carbon).
type Thresholds struct {
	WarnPct         float64 // percent of a limit at which a warning opens
	CarbonMonthlyKg float64 // kg CO2e per calendar month for the whole organization; 0 = no carbon alert
}

func (t Thresholds) warn() float64 {
	if t.WarnPct <= 0 || t.WarnPct >= BudgetCriticalPct {
		return DefaultWarnPct
	}
	return t.WarnPct
}

// CarbonSignal is the organization's emissions in the current calendar month so far (from the carbon module).
type CarbonSignal struct{ MonthToDateKg float64 }

// BudgetSignal is a budget with what was spent in its current period (supplied by the finops module through a port).
type BudgetSignal struct {
	ID        string
	ProjectID *string
	Amount    float64
	Spent     float64
	Currency  string
	Period    string
}

// ConnectionSignal is the health of a cloud connection (supplied by the cloudaccounts module through a port).
type ConnectionSignal struct {
	ID            string
	ProjectID     string
	Provider      string
	AccountRef    string
	SyncStatus    string
	LastError     string
	QualityStatus string // ok | mismatch | unchecked | "" (never synced)
	MismatchDays  int
}

// Conditions turns the current signals into the set of conditions that hold. Pure: the same input, the same result.
func Conditions(budgets []BudgetSignal, conns []ConnectionSignal, carbon *CarbonSignal, th Thresholds) []Condition {
	var out []Condition
	warnAt := th.warn()
	for _, b := range budgets {
		if b.Amount <= 0 {
			continue // a zero budget would be "infinitely exceeded" by any spend: not an alert, a misconfiguration
		}
		pct := b.Spent / b.Amount * 100
		if pct < warnAt {
			continue
		}
		sev, thr, what := Warning, warnAt, "is at"
		if pct >= BudgetCriticalPct {
			sev, thr, what = Critical, BudgetCriticalPct, "is over"
		}
		v := round1(pct)
		out = append(out, Condition{
			DedupeKey: "budget:" + b.ID, Kind: KindBudget, Severity: sev, ProjectID: b.ProjectID, ObjectRef: b.ID, Value: &v, Threshold: &thr,
			Title:  fmt.Sprintf("A %s budget %s %.0f%% (%.2f of %.2f %s)", b.Period, what, pct, b.Spent, b.Amount, b.Currency),
			Detail: "Spend counts the current budget period so far.",
		})
	}
	if carbon != nil && th.CarbonMonthlyKg > 0 {
		pct := carbon.MonthToDateKg / th.CarbonMonthlyKg * 100
		if pct >= warnAt {
			sev, thr, what := Warning, warnAt, "is at"
			if pct >= BudgetCriticalPct {
				sev, thr, what = Critical, BudgetCriticalPct, "is over"
			}
			v := round1(pct)
			out = append(out, Condition{
				DedupeKey: "carbon:month", Kind: KindCarbon, Severity: sev, Value: &v, Threshold: &thr,
				Title:  fmt.Sprintf("Carbon this month %s %.0f%% of the %.0f kg CO2e limit (%.1f kg so far)", what, pct, th.CarbonMonthlyKg, carbon.MonthToDateKg),
				Detail: "Operational emissions of the whole organization in the current calendar month.",
			})
		}
	}
	for _, c := range conns {
		pid := c.ProjectID
		if c.SyncStatus == "error" {
			out = append(out, Condition{
				DedupeKey: "connection:" + c.ID, Kind: KindConnectionError, Severity: Warning, ProjectID: &pid, ObjectRef: c.ID,
				Title:  fmt.Sprintf("%s connection %s needs attention", c.Provider, c.AccountRef),
				Detail: c.LastError,
			})
		}
		if c.QualityStatus == "mismatch" {
			d := float64(c.MismatchDays)
			out = append(out, Condition{
				DedupeKey: "quality:" + c.ID, Kind: KindDataQuality, Severity: Warning, ProjectID: &pid, ObjectRef: c.ID, Value: &d,
				Title:  fmt.Sprintf("%s connection %s: cost data does not match the cloud", c.Provider, c.AccountRef),
				Detail: fmt.Sprintf("%d days differ from the cloud's own totals; they are read again on the next sync.", c.MismatchDays),
			})
		}
	}
	return out
}

func round1(v float64) float64 { return float64(int64(v*10+0.5)) / 10 }

// Plan is what an evaluation has to do to the stored alerts.
type Plan struct {
	Open    []Condition // no live alert yet
	Refresh []Condition // a live alert exists: update value and severity, keep its status
	Resolve []string    // ids of live alerts whose condition no longer holds
}

// Reconcile compares the live alerts with the conditions that hold now.
func Reconcile(live []Alert, conds []Condition) Plan {
	byKey := map[string]Alert{}
	for _, a := range live {
		byKey[a.DedupeKey] = a
	}
	held := map[string]bool{}
	var p Plan
	for _, c := range conds {
		held[c.DedupeKey] = true
		if _, ok := byKey[c.DedupeKey]; ok {
			p.Refresh = append(p.Refresh, c)
		} else {
			p.Open = append(p.Open, c)
		}
	}
	for _, a := range live {
		if !held[a.DedupeKey] {
			p.Resolve = append(p.Resolve, a.ID)
		}
	}
	return p
}

// Transition validates a change a person asks for. Resolving is the evaluation's job only.
func Transition(from, to Status) error {
	ok := map[Status][]Status{
		New:          {Acknowledged, Muted},
		Acknowledged: {Muted},
		Muted:        {New},
	}
	for _, t := range ok[from] {
		if t == to {
			return nil
		}
	}
	return fmt.Errorf("%w: %s -> %s", ErrInvalidTransition, from, to)
}

// Repository is tenant-scoped (explicit tenant id and RLS).
type Repository interface {
	List(ctx context.Context, tenantID string, status string, limit int) ([]Alert, error)
	Live(ctx context.Context, tenantID string) ([]Alert, error)
	Open(ctx context.Context, tenantID string, c Condition, at time.Time) (Alert, bool, error)
	Refresh(ctx context.Context, tenantID string, c Condition, at time.Time) error
	Resolve(ctx context.Context, tenantID string, ids []string, at time.Time) error
	SetStatus(ctx context.Context, tenantID, id string, to Status, actor string) (Alert, error)
}
