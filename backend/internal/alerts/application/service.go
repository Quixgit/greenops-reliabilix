// Package application holds the alert use-cases: evaluation and the people's status changes.
package application

import (
	"context"
	"log/slog"
	"time"

	"github.com/quixgit/greenops-reliabilix/backend/internal/alerts/domain"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/notify"
)

// Ports implemented in the composition root from other modules (no cross-domain imports).
type BudgetSource interface {
	BudgetSignals(ctx context.Context, tenantID string) ([]domain.BudgetSignal, error)
}
type ConnectionSource interface {
	ConnectionSignals(ctx context.Context, tenantID string) ([]domain.ConnectionSignal, error)
}
type TenantLister interface {
	TenantIDs(ctx context.Context) ([]string, error)
}

type Service struct {
	Repo        domain.Repository
	Budgets     BudgetSource
	Connections ConnectionSource
	Tenants     TenantLister
	Notify      notify.Notifier // optional
	Log         *slog.Logger
	Now         func() time.Time
}

func (s Service) now() time.Time {
	if s.Now != nil {
		return s.Now().UTC()
	}
	return time.Now().UTC()
}

// Evaluate reconciles one organization's alerts with what is true now. A signal source that fails leaves the alerts
// of its kinds as they are: a failed read must never resolve alerts (that would hide a real problem).
func (s Service) Evaluate(ctx context.Context, tenantID string) error {
	live, err := s.Repo.Live(ctx, tenantID)
	if err != nil {
		return err
	}
	var budgets []domain.BudgetSignal
	var conns []domain.ConnectionSignal
	budgetsOK, connsOK := true, true
	if s.Budgets != nil {
		if budgets, err = s.Budgets.BudgetSignals(ctx, tenantID); err != nil {
			budgetsOK = false
			s.Log.Warn("alerts: budgets unavailable", "tenant", tenantID, "err", err)
		}
	}
	if s.Connections != nil {
		if conns, err = s.Connections.ConnectionSignals(ctx, tenantID); err != nil {
			connsOK = false
			s.Log.Warn("alerts: connections unavailable", "tenant", tenantID, "err", err)
		}
	}
	// Only the alerts of the sources that answered take part in the reconciliation.
	var scoped []domain.Alert
	for _, a := range live {
		if (a.Kind == domain.KindBudget && budgetsOK) || (a.Kind != domain.KindBudget && connsOK) {
			scoped = append(scoped, a)
		}
	}
	plan := domain.Reconcile(scoped, domain.Conditions(budgets, conns))
	at := s.now()
	for _, c := range plan.Open {
		a, created, err := s.Repo.Open(ctx, tenantID, c, at)
		if err != nil {
			return err
		}
		if created {
			s.announce(ctx, tenantID, a)
		}
	}
	for _, c := range plan.Refresh {
		if err := s.Repo.Refresh(ctx, tenantID, c, at); err != nil {
			return err
		}
	}
	return s.Repo.Resolve(ctx, tenantID, plan.Resolve, at)
}

// EvaluateAll runs Evaluate for every organization; one failing organization does not stop the rest.
func (s Service) EvaluateAll(ctx context.Context) (int, error) {
	ids, err := s.Tenants.TenantIDs(ctx)
	if err != nil {
		return 0, err
	}
	var first error
	done := 0
	for _, id := range ids {
		if err := s.Evaluate(ctx, id); err != nil {
			s.Log.Error("alerts: evaluation failed", "tenant", id, "err", err)
			if first == nil {
				first = err
			}
			continue
		}
		done++
	}
	return done, first
}

func (s Service) announce(ctx context.Context, tenantID string, a domain.Alert) {
	if s.Notify == nil {
		return
	}
	sev := notify.Warning
	if a.Severity == domain.Critical {
		sev = notify.Critical
	}
	_ = s.Notify.Publish(ctx, tenantID, notify.Event{Type: notify.TypeAlertOpened, Severity: sev, Title: a.Title, Body: a.Detail,
		Path: "/alerts", DedupeKey: "alert:" + a.ID, Attrs: map[string]string{"alert_id": a.ID, "kind": string(a.Kind)}})
}

func (s Service) List(ctx context.Context, tenantID, status string, limit int) ([]domain.Alert, error) {
	return s.Repo.List(ctx, tenantID, status, limit)
}

func (s Service) Acknowledge(ctx context.Context, tenantID, id, actor string) (domain.Alert, error) {
	return s.Repo.SetStatus(ctx, tenantID, id, domain.Acknowledged, actor)
}
func (s Service) Mute(ctx context.Context, tenantID, id, actor string) (domain.Alert, error) {
	return s.Repo.SetStatus(ctx, tenantID, id, domain.Muted, actor)
}
func (s Service) Unmute(ctx context.Context, tenantID, id, actor string) (domain.Alert, error) {
	return s.Repo.SetStatus(ctx, tenantID, id, domain.New, actor)
}
