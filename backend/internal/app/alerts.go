package app

import (
	"context"
	"time"

	alertsdomain "github.com/quixgit/greenops-reliabilix/backend/internal/alerts/domain"
	carbonapp "github.com/quixgit/greenops-reliabilix/backend/internal/carbon/application"
	clouddomain "github.com/quixgit/greenops-reliabilix/backend/internal/cloudaccounts/application"
	finopsapp "github.com/quixgit/greenops-reliabilix/backend/internal/finops/application"
	projectsapp "github.com/quixgit/greenops-reliabilix/backend/internal/projects/application"
)

// alertBudgets lets the alerts module read budget consumption without importing finops.
type alertBudgets struct{ finops finopsapp.Service }

func (a alertBudgets) BudgetSignals(ctx context.Context, tenantID string) ([]alertsdomain.BudgetSignal, error) {
	statuses, err := a.finops.Budgets(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	out := make([]alertsdomain.BudgetSignal, 0, len(statuses))
	for _, s := range statuses {
		out = append(out, alertsdomain.BudgetSignal{ID: s.ID, ProjectID: s.ProjectID, Amount: s.Amount, Spent: s.Spent, Currency: s.Currency, Period: s.Period})
	}
	return out, nil
}

// alertConnections lets the alerts module read connection health without importing cloudaccounts.
type alertConnections struct{ cloud clouddomain.Service }

func (a alertConnections) ConnectionSignals(ctx context.Context, tenantID string) ([]alertsdomain.ConnectionSignal, error) {
	conns, err := a.cloud.List(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	out := make([]alertsdomain.ConnectionSignal, 0, len(conns))
	for _, c := range conns {
		s := alertsdomain.ConnectionSignal{ID: c.ID, ProjectID: c.ProjectID, Provider: string(c.Provider), AccountRef: c.AccountRef, SyncStatus: string(c.SyncStatus)}
		if c.LastError != nil {
			s.LastError = *c.LastError
		}
		if c.LastQuality != nil {
			s.QualityStatus, s.MismatchDays = c.LastQuality.Status, c.LastQuality.MismatchDays
		}
		out = append(out, s)
	}
	return out, nil
}

// alertProjectCarbon reads each project's emissions this month and its limit without importing projects or carbon.
type alertProjectCarbon struct {
	projects projectsapp.Service
	carbon   *carbonapp.Service
}

func (a alertProjectCarbon) ProjectsMonthToDate(ctx context.Context, tenantID string, now time.Time) ([]alertsdomain.ProjectCarbonSignal, error) {
	limits, err := a.projects.CarbonLimits(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	from := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	out := make([]alertsdomain.ProjectCarbonSignal, 0, len(limits))
	for _, l := range limits {
		id := l.ProjectID
		s, err := a.carbon.Summary(ctx, tenantID, &id, from, now.Add(time.Hour))
		if err != nil {
			return nil, err
		}
		out = append(out, alertsdomain.ProjectCarbonSignal{ProjectID: l.ProjectID, Name: l.Name, MonthToDateKg: s.Current.CO2eKg, LimitKg: l.LimitKg})
	}
	return out, nil
}

// alertTenants lists every organization that has at least one project (the scheduler's fan-out).
type alertTenants struct{ projects projectsapp.Service }

func (a alertTenants) TenantIDs(ctx context.Context) ([]string, error) {
	refs, err := a.projects.ListForJobs(ctx)
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	var out []string
	for _, r := range refs {
		if !seen[r.TenantID] {
			seen[r.TenantID] = true
			out = append(out, r.TenantID)
		}
	}
	return out, nil
}

// alertCarbon lets the alerts module read this month's emissions without importing carbon.
type alertCarbon struct{ carbon *carbonapp.Service }

func (a alertCarbon) MonthToDate(ctx context.Context, tenantID string, now time.Time) (alertsdomain.CarbonSignal, error) {
	from := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	s, err := a.carbon.Summary(ctx, tenantID, nil, from, now.Add(time.Hour))
	if err != nil {
		return alertsdomain.CarbonSignal{}, err
	}
	return alertsdomain.CarbonSignal{MonthToDateKg: s.Current.CO2eKg}, nil
}
