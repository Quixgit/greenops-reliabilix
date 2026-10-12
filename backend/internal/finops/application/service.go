// Package application holds the finops use-cases.
package application

import (
	"context"
	"time"

	"github.com/quixgit/greenops-reliabilix/backend/internal/finops/domain"
)

// ExistingSource tells the commitments report what the organization's clouds already hold (a port; the composition root wires the cloud
// accounts module to it).
type ExistingSource interface {
	ExistingCommitments(ctx context.Context, tenantID string) ([]domain.Existing, error)
}

type Service struct {
	Repo domain.Repository
	// Existing is optional: without it nothing is known about commitments already held.
	Existing ExistingSource
	Now      func() time.Time
}

func (s Service) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now()
}

// RateSource returns the reference rates on or after a day.
type RateSource interface {
	Fetch(ctx context.Context, since time.Time) ([]domain.FXRate, error)
}

// RefreshRates stores the rates newer than what is stored (all of them the first time). A week of overlap is kept so a
// correction published for a recent day is picked up.
func (s Service) RefreshRates(ctx context.Context, src RateSource) (int, error) {
	latest, err := s.Repo.LatestRateDay(ctx)
	if err != nil {
		return 0, err
	}
	since := time.Time{}
	if !latest.IsZero() {
		since = latest.AddDate(0, 0, -7)
	}
	rates, err := src.Fetch(ctx, since)
	if err != nil {
		return 0, err
	}
	return len(rates), s.Repo.SaveRates(ctx, rates)
}

func (s Service) Summary(ctx context.Context, tenantID string, project *string, from, to time.Time) (domain.Summary, error) {
	return s.Repo.Summary(ctx, tenantID, project, from, to)
}

func (s Service) CreateBudget(ctx context.Context, tenantID string, b domain.Budget) (domain.Budget, error) {
	if err := b.Validate(); err != nil {
		return domain.Budget{}, err
	}
	return s.Repo.CreateBudget(ctx, tenantID, b)
}

func (s Service) DeleteBudget(ctx context.Context, tenantID, id string) error {
	return s.Repo.DeleteBudget(ctx, tenantID, id)
}

// Budgets lists budgets with their consumption in the current period.
func (s Service) Budgets(ctx context.Context, tenantID string) ([]domain.BudgetStatus, error) {
	bs, err := s.Repo.ListBudgets(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	now := s.now()
	out := make([]domain.BudgetStatus, 0, len(bs))
	for _, b := range bs {
		start, end := domain.PeriodBounds(b.Period, now)
		spent, err := s.Repo.Spent(ctx, tenantID, b.ProjectID, b.Currency, start, end)
		if err != nil {
			return nil, err
		}
		out = append(out, domain.EvaluateWith(b, spent, now, s.periodForecast(ctx, tenantID, b.ProjectID, b.Currency, start, end, now)))
	}
	return out, nil
}

// Anomalies lists the unusual days and, for each, what moved (at most 10 days are explained, the latest first).
func (s Service) Anomalies(ctx context.Context, tenantID string, project *string, from, to time.Time) ([]domain.Anomaly, error) {
	as, err := s.Repo.Anomalies(ctx, tenantID, project, from, to)
	if err != nil {
		return nil, err
	}
	for i := len(as) - 1; i >= 0 && len(as)-i <= 10; i-- {
		d, err := s.Repo.AnomalyDrivers(ctx, tenantID, project, as[i].Day, 3)
		if err != nil {
			return nil, err
		}
		as[i].Drivers = d
	}
	for i := range as {
		if as[i].Drivers == nil {
			as[i].Drivers = []domain.Driver{}
		}
	}
	return as, nil
}

// historyDays is how far back the weekday pattern looks: six weeks plus the two weeks the trend compares.
const historyDays = 56

// periodForecast forecasts the cost of [start, end) from the history before now; nil when there is no data to go by (the caller keeps
// its linear projection).
func (s Service) periodForecast(ctx context.Context, tenantID string, project *string, currency string, start, end, now time.Time) *domain.Forecast {
	from := now.UTC().Truncate(24*time.Hour).AddDate(0, 0, -historyDays)
	if start.Before(from) {
		from = start
	}
	pts, _, err := s.Repo.DailyCost(ctx, tenantID, project, currency, from, now.UTC().Truncate(24*time.Hour).AddDate(0, 0, 1))
	if err != nil || len(pts) == 0 {
		return nil
	}
	f := domain.BuildForecast(pts, start, end, now)
	return &f
}

// Forecast is the expected cost at the end of the current month for the whole organization or one project, in the reporting currency.
func (s Service) Forecast(ctx context.Context, tenantID string, project *string) (domain.Forecast, string, error) {
	now := s.now()
	start, end := domain.PeriodBounds("monthly", now)
	from := now.UTC().Truncate(24*time.Hour).AddDate(0, 0, -historyDays)
	if start.Before(from) {
		from = start
	}
	pts, cur, err := s.Repo.DailyCost(ctx, tenantID, project, "", from, now.UTC().Truncate(24*time.Hour).AddDate(0, 0, 1))
	if err != nil {
		return domain.Forecast{}, "", err
	}
	return domain.BuildForecast(pts, start, end, now), cur, nil
}

// Commitments reports what committing the steady part of the compute and database spend would save (ADR-0051). The window ends at the
// start of today, so a day still being loaded never lowers the baseline.
func (s Service) Commitments(ctx context.Context, tenantID string, project *string, p domain.CommitmentParams) (domain.Commitments, error) {
	end := s.now().UTC().Truncate(24 * time.Hour)
	p.To, p.From = end, end.AddDate(0, 0, -p.LookbackDays)
	days, cur, err := s.Repo.CommittableDaily(ctx, tenantID, project, p.From, p.To)
	if err != nil {
		return domain.Commitments{}, err
	}
	p.Currency = cur
	if s.Existing != nil {
		if p.Read, err = s.Existing.ExistingCommitments(ctx, tenantID); err != nil {
			return domain.Commitments{}, err
		}
	}
	return domain.BuildCommitments(days, p), nil
}

// TagKeys lists the tag keys with cost in the window and what share of the bill each explains.
func (s Service) TagKeys(ctx context.Context, tenantID string, project *string, from, to time.Time) ([]domain.TagKeyInfo, string, error) {
	return s.Repo.TagKeys(ctx, tenantID, project, from, to)
}

// Showback splits the window by the values of one tag key.
func (s Service) Showback(ctx context.Context, tenantID string, project *string, tag string, from, to time.Time) (domain.Showback, error) {
	return s.Repo.Showback(ctx, tenantID, project, tag, from, to)
}
