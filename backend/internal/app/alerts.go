package app

import (
	"context"

	alertsdomain "github.com/quixgit/greenops-reliabilix/backend/internal/alerts/domain"
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
