package application

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/settings"
	"github.com/quixgit/greenops-reliabilix/backend/internal/settings/domain"
)

type memRepo struct {
	platform map[string]domain.Stored
	tenants  map[string]map[string]domain.Stored
	history  []settings.HistoryEntry
	reads    int
	fail     error
}

func newRepo() *memRepo {
	return &memRepo{platform: map[string]domain.Stored{}, tenants: map[string]map[string]domain.Stored{}}
}

func (m *memRepo) PlatformValues(context.Context) (map[string]domain.Stored, error) {
	m.reads++
	if m.fail != nil {
		return nil, m.fail
	}
	return clone(m.platform), nil
}
func (m *memRepo) TenantValues(_ context.Context, t string) (map[string]domain.Stored, error) {
	m.reads++
	if m.fail != nil {
		return nil, m.fail
	}
	return clone(m.tenants[t]), nil
}
func clone(in map[string]domain.Stored) map[string]domain.Stored {
	out := map[string]domain.Stored{}
	for k, v := range in {
		out[k] = v
	}
	return out
}
func (m *memRepo) ApplyTenant(_ context.Context, t, actor string, ch []domain.Change) error {
	if m.tenants[t] == nil {
		m.tenants[t] = map[string]domain.Stored{}
	}
	for _, c := range ch {
		if c.New == nil {
			delete(m.tenants[t], c.Key)
		} else {
			m.tenants[t][c.Key] = domain.Stored{Value: c.New, Source: settings.FromTenant, UpdatedBy: actor, UpdatedAt: time.Now()}
		}
		m.history = append(m.history, settings.HistoryEntry{Key: c.Key, Old: c.Old, New: c.New, Actor: actor, Via: "tenant"})
	}
	return nil
}
func (m *memRepo) TenantHistory(context.Context, string, int) ([]settings.HistoryEntry, error) {
	return m.history, nil
}
func (m *memRepo) OperatorSetPlatform(_ context.Context, actor, key string, v json.RawMessage, reset bool) error {
	if reset {
		delete(m.platform, key)
		return nil
	}
	m.platform[key] = domain.Stored{Value: v, Source: settings.FromPlatform, UpdatedBy: actor, UpdatedAt: time.Now()}
	return nil
}
func (m *memRepo) OperatorTenantValues(_ context.Context, t string) (map[string]domain.Stored, error) {
	return clone(m.tenants[t]), nil
}
func (m *memRepo) OperatorSetTenant(_ context.Context, actor, t, key string, v json.RawMessage, reset bool) (bool, error) {
	if t == "missing" {
		return false, nil
	}
	if m.tenants[t] == nil {
		m.tenants[t] = map[string]domain.Stored{}
	}
	if reset {
		delete(m.tenants[t], key)
		return true, nil
	}
	m.tenants[t][key] = domain.Stored{Value: v, Source: settings.FromOperator, UpdatedBy: actor, UpdatedAt: time.Now()}
	return true, nil
}
func (m *memRepo) OperatorHistory(context.Context, *string, int) ([]settings.HistoryEntry, error) {
	return m.history, nil
}

func raw(s string) json.RawMessage { return json.RawMessage(s) }

func newSvc() (*Service, *memRepo, *time.Time) {
	repo := newRepo()
	now := time.Date(2026, 10, 9, 12, 0, 0, 0, time.UTC)
	return &Service{Repo: repo, Now: func() time.Time { return now }}, repo, &now
}

var ctx = context.Background()

func TestInheritanceOrder(t *testing.T) {
	s, repo, _ := newSvc()
	k := settings.KeyInviteExpiryDays
	if got := s.Int(ctx, "t1", k); got != 7 {
		t.Fatalf("default = %d", got)
	}
	if err := s.SetPlatform(ctx, "op", map[string]json.RawMessage{k: raw("10")}); err != nil {
		t.Fatal(err)
	}
	if got := s.Int(ctx, "t1", k); got != 10 {
		t.Errorf("the platform value is every organization's default, got %d", got)
	}
	if err := s.SetTenant(ctx, "t1", "alice", map[string]json.RawMessage{k: raw("3")}); err != nil {
		t.Fatal(err)
	}
	if got, other := s.Int(ctx, "t1", k), s.Int(ctx, "t2", k); got != 3 || other != 10 {
		t.Errorf("own value wins only for its organization: t1=%d t2=%d", got, other)
	}
	if err := s.SetTenantOverride(ctx, "op", "t1", map[string]json.RawMessage{k: raw("5")}); err != nil {
		t.Fatal(err)
	}
	if got := s.Int(ctx, "t1", k); got != 5 {
		t.Errorf("operator override wins, got %d", got)
	}
	if repo.tenants["t1"][k].Source != settings.FromOperator {
		t.Error("an override must be stored as operator-sourced (locked)")
	}
	if err := s.ClearTenantOverride(ctx, "op", "t1", []string{k}); err != nil {
		t.Fatal(err)
	}
	if got := s.Int(ctx, "t1", k); got != 10 {
		t.Errorf("clearing the override falls back to the platform value, got %d", got)
	}
}

func TestOrganizationCannotTouchLockedOrForeignScopes(t *testing.T) {
	s, _, _ := newSvc()
	k := settings.KeyInviteExpiryDays
	_ = s.SetTenantOverride(ctx, "op", "t1", map[string]json.RawMessage{k: raw("5")})
	err := s.SetTenant(ctx, "t1", "alice", map[string]json.RawMessage{k: raw("20")})
	var inv InvalidError
	if !errors.As(err, &inv) || inv.Problems[k] != domain.ErrLocked.Error() {
		t.Fatalf("locked key: %v", err)
	}
	if err := s.ResetTenant(ctx, "t1", "alice", []string{k}); !errors.Is(err, settings.ErrInvalid) {
		t.Errorf("reset of a locked key: %v", err)
	}
	for _, key := range []string{settings.KeyLimitProjects, settings.KeyMaintenanceEnabled, settings.KeyModuleAutomation, "no.such_key"} {
		v := raw("1")
		if key == settings.KeyMaintenanceEnabled || key == settings.KeyModuleAutomation {
			v = raw("true")
		}
		if err := s.SetTenant(ctx, "t1", "alice", map[string]json.RawMessage{key: v}); !errors.Is(err, settings.ErrInvalid) {
			t.Errorf("%s: an organization must not set operator/platform/unknown settings, got %v", key, err)
		}
	}
	if err := s.SetTenantOverride(ctx, "op", "t1", map[string]json.RawMessage{settings.KeyMaintenanceEnabled: raw("true")}); !errors.Is(err, settings.ErrInvalid) {
		t.Errorf("a platform-wide setting cannot be overridden per organization: %v", err)
	}
	if err := s.SetTenantOverride(ctx, "op", "missing", map[string]json.RawMessage{settings.KeyLimitProjects: raw("5")}); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("unknown organization: %v", err)
	}
}

func TestValidationIsAllOrNothing(t *testing.T) {
	s, repo, _ := newSvc()
	err := s.SetTenant(ctx, "t1", "alice", map[string]json.RawMessage{
		settings.KeyInviteExpiryDays: raw("14"),   // fine
		settings.KeyAPIKeysMaxActive: raw("9999"), // out of range
	})
	var inv InvalidError
	if !errors.As(err, &inv) || len(inv.Problems) != 1 || inv.Problems[settings.KeyAPIKeysMaxActive] == "" {
		t.Fatalf("err = %v", err)
	}
	if len(repo.tenants["t1"]) != 0 || len(repo.history) != 0 {
		t.Error("nothing may be saved when any value is rejected")
	}
	if err := s.SetTenant(ctx, "t1", "alice", map[string]json.RawMessage{settings.KeyReportFormats: raw("[]")}); !errors.Is(err, settings.ErrInvalid) {
		t.Errorf("an empty format list must be refused: %v", err)
	}
}

func TestChangesAreRecordedAndNoOpsAreNot(t *testing.T) {
	s, repo, _ := newSvc()
	k := settings.KeyAPIKeysMaxActive
	_ = s.SetTenant(ctx, "t1", "alice", map[string]json.RawMessage{k: raw("5")})
	_ = s.SetTenant(ctx, "t1", "alice", map[string]json.RawMessage{k: raw(" 5 ")}) // same value, other spelling
	if len(repo.history) != 1 {
		t.Fatalf("history = %d entries, want 1", len(repo.history))
	}
	_ = s.SetTenant(ctx, "t1", "bob", map[string]json.RawMessage{k: raw("8")})
	h := repo.history[1]
	if string(h.Old) != "5" || string(h.New) != "8" || h.Actor != "bob" {
		t.Errorf("history entry = %+v", h)
	}
	_ = s.ResetTenant(ctx, "t1", "bob", []string{k})
	if last := repo.history[2]; last.New != nil || string(last.Old) != "8" {
		t.Errorf("reset entry = %+v", last)
	}
	if got := s.Int(ctx, "t1", k); got != 10 {
		t.Errorf("after reset the default applies, got %d", got)
	}
	n := len(repo.history)
	_ = s.ResetTenant(ctx, "t1", "bob", []string{k}) // nothing to reset
	if len(repo.history) != n {
		t.Error("resetting a key without an own value must not create history")
	}
}

func TestReadsAreCachedBrieflyAndInvalidatedByWrites(t *testing.T) {
	s, repo, now := newSvc()
	k := settings.KeyAPIKeysMaxActive
	s.Int(ctx, "t1", k)
	reads := repo.reads
	for range 20 {
		s.Int(ctx, "t1", k)
	}
	if repo.reads != reads {
		t.Errorf("hot path hit the database %d extra times", repo.reads-reads)
	}
	_ = s.SetTenant(ctx, "t1", "alice", map[string]json.RawMessage{k: raw("3")})
	if got := s.Int(ctx, "t1", k); got != 3 {
		t.Errorf("a write must be visible immediately in this process, got %d", got)
	}
	repo.tenants["t1"][k] = domain.Stored{Value: raw("4"), Source: settings.FromTenant} // changed by another process
	if got := s.Int(ctx, "t1", k); got != 3 {
		t.Errorf("within the TTL the cached value is served, got %d", got)
	}
	*now = now.Add(cacheTTL + time.Second)
	if got := s.Int(ctx, "t1", k); got != 4 {
		t.Errorf("after the TTL the change from another process shows up, got %d", got)
	}
}

func TestReadsFallBackToDefaultsWhenTheStoreFails(t *testing.T) {
	s, repo, _ := newSvc()
	repo.fail = errors.New("database down")
	if s.Int(ctx, "t1", settings.KeyAPIKeysMaxActive) != 10 || !s.Bool(ctx, "t1", settings.KeyNotifEnabled) || s.String(ctx, "", settings.KeyMaintenanceMessage) == "" {
		t.Error("a failing store must yield the catalog defaults, not errors or zero values")
	}
	if len(s.Strings(ctx, "t1", settings.KeyReportFormats)) != 3 {
		t.Error("list defaults must survive too")
	}
}

func TestACorruptStoredValueIsIgnored(t *testing.T) {
	s, repo, _ := newSvc()
	repo.platform[settings.KeyNotifMaxAttempts] = domain.Stored{Value: raw(`"lots"`), Source: settings.FromPlatform}
	if got := s.Int(ctx, "", settings.KeyNotifMaxAttempts); got != 8 {
		t.Errorf("a stored value that no longer validates must not be used, got %d", got)
	}
}

func TestViews(t *testing.T) {
	s, _, _ := newSvc()
	_ = s.SetPlatform(ctx, "op", map[string]json.RawMessage{settings.KeyInviteExpiryDays: raw("9"), settings.KeyLimitProjects: raw("50")})
	_ = s.SetTenantOverride(ctx, "op", "t1", map[string]json.RawMessage{settings.KeyAPIKeysMaxActive: raw("2")})

	items, err := s.TenantItems(ctx, "t1", true)
	if err != nil {
		t.Fatal(err)
	}
	by := map[string]settings.Item{}
	for _, it := range items {
		by[it.Key] = it
		if it.Scope == settings.ScopePlatform {
			t.Errorf("organizations must not see platform-scope setting %s", it.Key)
		}
	}
	if it := by[settings.KeyInviteExpiryDays]; it.Source != settings.FromPlatform || !it.Editable || string(it.Value) != "9" {
		t.Errorf("inherited: %+v", it)
	}
	if it := by[settings.KeyAPIKeysMaxActive]; !it.Locked || it.Editable || it.Source != settings.FromOperator {
		t.Errorf("locked: %+v", it)
	}
	if it := by[settings.KeyLimitProjects]; it.Editable || string(it.Value) != "50" {
		t.Errorf("operator-managed settings are read-only for the organization: %+v", it)
	}
	if ro, _ := s.TenantItems(ctx, "t1", false); func() bool {
		for _, it := range ro {
			if it.Editable {
				return true
			}
		}
		return false
	}() {
		t.Error("without write permission nothing is editable")
	}

	plat, _ := s.PlatformItems(ctx)
	scopes := map[settings.Scope]bool{}
	for _, it := range plat {
		scopes[it.Scope] = true
	}
	if len(scopes) != 3 {
		t.Error("operators see every scope at platform level")
	}
	ot, _ := s.OperatorTenantItems(ctx, "t1")
	for _, it := range ot {
		if it.Scope == settings.ScopePlatform {
			t.Errorf("per-organization view must not list platform-scope %s", it.Key)
		}
	}
}
