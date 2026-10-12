// Package domain holds cost-side concepts, kept separate from carbon.
package domain

import (
	"context"
	"errors"
	"time"
)

type Summary struct {
	HasData           bool    `json:"has_data"`
	TotalCost         float64 `json:"total_cost"`
	PreviousTotalCost float64 `json:"previous_total_cost"`
	// PreviousComparable is false when the current or previous window has data for fewer than 80% of its
	// days: comparing them would show a fake change, so clients must not draw a delta.
	PreviousComparable bool   `json:"previous_comparable"`
	Currency           string `json:"currency"`
	ByService          []Line `json:"by_service"`
	// Excluded lists currencies whose amounts could not be converted into Currency (no reference rate): they are not in
	// the totals, and the client says so.
	Excluded []Excluded `json:"excluded"`
}

// Excluded is a currency left out of the totals and how many cost rows it has in the window.
type Excluded struct {
	Currency string `json:"currency"`
	Rows     int    `json:"rows"`
}

// FXRate is the value of one euro in a currency on a day (ECB reference rate).
type FXRate struct {
	Day      time.Time
	Currency string
	PerEUR   float64
}

type Line struct {
	Service string  `json:"service"`
	Cost    float64 `json:"cost"`
}

type Budget struct {
	ID        string    `json:"id"`
	ProjectID *string   `json:"project_id"` // nil = whole tenant
	Amount    float64   `json:"amount"`
	Currency  string    `json:"currency"`
	Period    string    `json:"period"` // monthly | quarterly | yearly
	CreatedAt time.Time `json:"created_at"`
}

// BudgetStatus is a budget with its consumption in the current period.
type BudgetStatus struct {
	Budget
	PeriodStart time.Time `json:"period_start"`
	PeriodEnd   time.Time `json:"period_end"`
	Spent       float64   `json:"spent"`
	UsedPct     float64   `json:"used_pct"`
	Projected   float64   `json:"projected"` // expected cost at the end of the period (seasonal forecast, or linear when the history is short)
	// ProjectedLow and ProjectedHigh bound it (an 80% range); ForecastMethod says which method gave it.
	ProjectedLow   float64 `json:"projected_low"`
	ProjectedHigh  float64 `json:"projected_high"`
	ForecastMethod string  `json:"forecast_method"`
	State          string  `json:"state"` // ok | at_risk | exceeded
}

type Anomaly struct {
	Day    time.Time `json:"day"`
	Cost   float64   `json:"cost"`
	Mean   float64   `json:"expected"`
	Stddev float64   `json:"stddev"`
	// Drivers are the services and regions that moved the day the most, biggest increase first (ADR-0053).
	Drivers []Driver `json:"drivers"`
}

// Driver is one service in one region that cost more on an anomalous day than on a usual one.
type Driver struct {
	Service  string  `json:"service"`
	Region   string  `json:"region"`
	Cost     float64 `json:"cost"`     // on the day
	Expected float64 `json:"expected"` // the daily average of the 14 days before
	Delta    float64 `json:"delta"`
	New      bool    `json:"new"` // nothing like it before: a new service or region
}

var (
	ErrInvalidBudget = errors.New("invalid budget")
	ErrNotFound      = errors.New("not found")
)

// Validate checks a budget definition.
func (b Budget) Validate() error {
	if b.Amount < 0 || len(b.Currency) != 3 {
		return ErrInvalidBudget
	}
	switch b.Period {
	case "monthly", "quarterly", "yearly":
		return nil
	}
	return ErrInvalidBudget
}

// PeriodBounds returns [start, end) of the budget period containing now.
func PeriodBounds(period string, now time.Time) (time.Time, time.Time) {
	now = now.UTC()
	y, m := now.Year(), now.Month()
	switch period {
	case "yearly":
		return time.Date(y, 1, 1, 0, 0, 0, 0, time.UTC), time.Date(y+1, 1, 1, 0, 0, 0, 0, time.UTC)
	case "quarterly":
		qm := time.Month((int(m)-1)/3*3 + 1)
		return time.Date(y, qm, 1, 0, 0, 0, 0, time.UTC), time.Date(y, qm+3, 1, 0, 0, 0, 0, time.UTC)
	default:
		return time.Date(y, m, 1, 0, 0, 0, 0, time.UTC), time.Date(y, m+1, 1, 0, 0, 0, 0, time.UTC)
	}
}

// Evaluate computes consumption, a linear projection and the state: exceeded (spent > amount),
// at_risk (projected > amount or >= 80% used), otherwise ok.
func Evaluate(b Budget, spent float64, now time.Time) BudgetStatus {
	return EvaluateWith(b, spent, now, nil)
}

// EvaluateWith is Evaluate with the seasonal forecast of the period when there is one (nil: the linear projection).
func EvaluateWith(b Budget, spent float64, now time.Time, f *Forecast) BudgetStatus {
	start, end := PeriodBounds(b.Period, now)
	elapsed := now.UTC().Sub(start)
	if elapsed < 24*time.Hour {
		elapsed = 24 * time.Hour
	}
	s := BudgetStatus{Budget: b, PeriodStart: start, PeriodEnd: end, Spent: spent}
	s.Projected = spent * float64(end.Sub(start)) / float64(elapsed)
	s.ProjectedLow, s.ProjectedHigh, s.ForecastMethod = s.Projected, s.Projected, "linear"
	if f != nil {
		s.Projected, s.ProjectedLow, s.ProjectedHigh, s.ForecastMethod = f.Expected, f.Low, f.High, f.Method
	}
	if b.Amount > 0 {
		s.UsedPct = spent / b.Amount * 100
	}
	switch {
	case spent > b.Amount:
		s.State = "exceeded"
	case s.Projected > b.Amount || s.UsedPct >= 80:
		s.State = "at_risk"
	default:
		s.State = "ok"
	}
	return s
}

type Repository interface {
	// CommittableDaily is the compute and database spend per provider and day (ADR-0051).
	CommittableDaily(ctx context.Context, tenantID string, project *string, from, to time.Time) ([]DayCost, string, error)
	LatestRateDay(ctx context.Context) (time.Time, error)
	SaveRates(ctx context.Context, rates []FXRate) error
	Summary(ctx context.Context, tenantID string, project *string, from, to time.Time) (Summary, error)
	ListBudgets(ctx context.Context, tenantID string) ([]Budget, error)
	CreateBudget(ctx context.Context, tenantID string, b Budget) (Budget, error)
	DeleteBudget(ctx context.Context, tenantID, id string) error
	Spent(ctx context.Context, tenantID string, project *string, currency string, from, to time.Time) (float64, error)
	Anomalies(ctx context.Context, tenantID string, project *string, from, to time.Time) ([]Anomaly, error)
	// AnomalyDrivers lists what moved on a day (ADR-0053).
	AnomalyDrivers(ctx context.Context, tenantID string, project *string, day time.Time, limit int) ([]Driver, error)
	// DailyCost is the cost per day of [from, to) in the currency given or, when empty, the reporting currency.
	DailyCost(ctx context.Context, tenantID string, project *string, currency string, from, to time.Time) ([]DayPoint, string, error)
	// TagKeys lists the tag keys with cost in [from, to) and how much of the bill each explains (ADR-0061).
	TagKeys(ctx context.Context, tenantID string, project *string, from, to time.Time) ([]TagKeyInfo, string, error)
	// Showback splits [from, to) by the values of one tag key, in the reporting currency.
	Showback(ctx context.Context, tenantID string, project *string, tag string, from, to time.Time) (Showback, error)
}
