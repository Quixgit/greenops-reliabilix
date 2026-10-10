package application

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/quixgit/greenops-reliabilix/backend/internal/alerts/domain"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/notify"
)

type memRepo struct {
	domain.Repository
	live      []domain.Alert
	opened    []domain.Condition
	refreshed []domain.Condition
	resolved  []string
}

func (m *memRepo) Live(context.Context, string) ([]domain.Alert, error) { return m.live, nil }
func (m *memRepo) Open(_ context.Context, _ string, c domain.Condition, _ time.Time) (domain.Alert, bool, error) {
	m.opened = append(m.opened, c)
	return domain.Alert{ID: "new-" + c.DedupeKey, Title: c.Title, Severity: c.Severity, Kind: c.Kind}, true, nil
}
func (m *memRepo) Refresh(_ context.Context, _ string, c domain.Condition, _ time.Time) error {
	m.refreshed = append(m.refreshed, c)
	return nil
}
func (m *memRepo) Resolve(_ context.Context, _ string, ids []string, _ time.Time) error {
	m.resolved = append(m.resolved, ids...)
	return nil
}

type budgets struct {
	sig []domain.BudgetSignal
	err error
}

func (b budgets) BudgetSignals(context.Context, string) ([]domain.BudgetSignal, error) { return b.sig, b.err }

type conns struct {
	sig []domain.ConnectionSignal
	err error
}

func (c conns) ConnectionSignals(context.Context, string) ([]domain.ConnectionSignal, error) {
	return c.sig, c.err
}

type notes struct{ events []notify.Event }

func (n *notes) Publish(_ context.Context, _ string, e notify.Event) error {
	n.events = append(n.events, e)
	return nil
}

func svc(r *memRepo, b budgets, c conns, n notify.Notifier) Service {
	return Service{Repo: r, Budgets: b, Connections: c, Notify: n, Log: slog.New(slog.NewTextHandler(io.Discard, nil)),
		Now: func() time.Time { return time.Date(2026, 10, 10, 0, 0, 0, 0, time.UTC) }}
}

func TestEvaluateOpensNotifiesAndResolves(t *testing.T) {
	r := &memRepo{live: []domain.Alert{{ID: "old", Kind: domain.KindConnectionError, DedupeKey: "connection:gone"}}}
	n := &notes{}
	b := budgets{sig: []domain.BudgetSignal{{ID: "b1", Amount: 100, Spent: 120, Currency: "USD", Period: "monthly"}}}
	if err := svc(r, b, conns{}, n).Evaluate(context.Background(), "t"); err != nil {
		t.Fatal(err)
	}
	if len(r.opened) != 1 || r.opened[0].DedupeKey != "budget:b1" {
		t.Fatalf("opened %+v", r.opened)
	}
	if len(r.resolved) != 1 || r.resolved[0] != "old" {
		t.Fatalf("the connection condition cleared, its alert must resolve: %v", r.resolved)
	}
	if len(n.events) != 1 || n.events[0].Severity != notify.Critical || n.events[0].Type != notify.TypeAlertOpened {
		t.Fatalf("one critical notification expected: %+v", n.events)
	}
}

func TestAFailedSourceNeverResolvesItsAlerts(t *testing.T) {
	r := &memRepo{live: []domain.Alert{
		{ID: "bud", Kind: domain.KindBudget, DedupeKey: "budget:x"},
		{ID: "con", Kind: domain.KindConnectionError, DedupeKey: "connection:y"},
	}}
	err := svc(r, budgets{err: errors.New("db down")}, conns{}, nil).Evaluate(context.Background(), "t")
	if err != nil {
		t.Fatal(err)
	}
	if len(r.resolved) != 1 || r.resolved[0] != "con" {
		t.Fatalf("only the connection alert may resolve (budgets could not be read): %v", r.resolved)
	}
}

func TestExistingAlertIsRefreshedNotReopened(t *testing.T) {
	r := &memRepo{live: []domain.Alert{{ID: "1", Kind: domain.KindBudget, DedupeKey: "budget:b1", Status: domain.Muted}}}
	n := &notes{}
	b := budgets{sig: []domain.BudgetSignal{{ID: "b1", Amount: 100, Spent: 90, Currency: "USD", Period: "monthly"}}}
	if err := svc(r, b, conns{}, n).Evaluate(context.Background(), "t"); err != nil {
		t.Fatal(err)
	}
	if len(r.opened) != 0 || len(r.refreshed) != 1 || len(n.events) != 0 {
		t.Fatalf("a muted alert must stay quiet: opened=%d refreshed=%d events=%d", len(r.opened), len(r.refreshed), len(n.events))
	}
}
