// Package application resolves and changes settings: inheritance (default, platform, organization, operator lock),
// validation against the catalog, caching for the hot read path, and the rules about who may change what.
package application

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"slices"
	"sort"
	"sync"
	"time"

	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/settings"
	"github.com/quixgit/greenops-reliabilix/backend/internal/settings/domain"
)

const cacheTTL = 10 * time.Second

type cached struct {
	vals map[string]domain.Stored
	at   time.Time
}

// Service implements settings.Reader for the domains and the management API for people.
type Service struct {
	Repo    domain.Repository
	Catalog *settings.Catalog
	Log     *slog.Logger
	Now     func() time.Time

	mu       sync.Mutex
	platform *cached
	tenants  map[string]*cached
}

var _ settings.Reader = (*Service)(nil)

func (s *Service) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now()
}

func (s *Service) cat() *settings.Catalog {
	if s.Catalog != nil {
		return s.Catalog
	}
	return settings.Default()
}

func (s *Service) warn(msg string, args ...any) {
	if s.Log != nil {
		s.Log.Warn(msg, args...)
	}
}

// ---- reads (cached) ---------------------------------------------------------------------------------------------

func (s *Service) platformValues(ctx context.Context) (map[string]domain.Stored, error) {
	s.mu.Lock()
	if c := s.platform; c != nil && s.now().Sub(c.at) < cacheTTL {
		s.mu.Unlock()
		return c.vals, nil
	}
	s.mu.Unlock()
	vals, err := s.Repo.PlatformValues(ctx)
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	s.platform = &cached{vals: vals, at: s.now()}
	s.mu.Unlock()
	return vals, nil
}

func (s *Service) tenantValues(ctx context.Context, tenantID string) (map[string]domain.Stored, error) {
	s.mu.Lock()
	if c := s.tenants[tenantID]; c != nil && s.now().Sub(c.at) < cacheTTL {
		s.mu.Unlock()
		return c.vals, nil
	}
	s.mu.Unlock()
	vals, err := s.Repo.TenantValues(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	if s.tenants == nil || len(s.tenants) > 5000 {
		s.tenants = map[string]*cached{}
	}
	s.tenants[tenantID] = &cached{vals: vals, at: s.now()}
	s.mu.Unlock()
	return vals, nil
}

func (s *Service) invalidate(tenantID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if tenantID == "" {
		s.platform = nil
		s.tenants = nil // a platform default can change every organization's effective value
		return
	}
	delete(s.tenants, tenantID)
}

// value returns the effective typed value. Any failure falls back to the catalog default.
func (s *Service) value(ctx context.Context, tenantID, key string) any {
	d, ok := s.cat().Lookup(key)
	if !ok {
		panic("settings: unknown key " + key)
	}
	if raw, _, found := s.resolve(ctx, d, tenantID); found {
		if _, v, err := d.Validate(raw); err == nil {
			return v
		}
	}
	return d.Default
}

// resolve finds the stored value that applies, if any.
func (s *Service) resolve(ctx context.Context, d *settings.Def, tenantID string) (json.RawMessage, settings.Source, bool) {
	if d.Scope != settings.ScopePlatform && tenantID != "" {
		if tv, err := s.tenantValues(ctx, tenantID); err != nil {
			s.warn("settings: organization values unavailable, using defaults", "err", err)
		} else if v, ok := tv[d.Key]; ok {
			return v.Value, v.Source, true
		}
	}
	pv, err := s.platformValues(ctx)
	if err != nil {
		s.warn("settings: platform values unavailable, using defaults", "err", err)
		return nil, settings.FromDefault, false
	}
	if v, ok := pv[d.Key]; ok {
		return v.Value, settings.FromPlatform, true
	}
	return nil, settings.FromDefault, false
}

func (s *Service) Bool(ctx context.Context, tenantID, key string) bool {
	v, _ := s.value(ctx, tenantID, key).(bool)
	return v
}
func (s *Service) Int(ctx context.Context, tenantID, key string) int64 {
	v, _ := s.value(ctx, tenantID, key).(int64)
	return v
}
func (s *Service) String(ctx context.Context, tenantID, key string) string {
	v, _ := s.value(ctx, tenantID, key).(string)
	return v
}
func (s *Service) Strings(ctx context.Context, tenantID, key string) []string {
	v, _ := s.value(ctx, tenantID, key).([]string)
	return append([]string(nil), v...)
}

// ---- views -----------------------------------------------------------------------------------------------------

// Groups returns the groups in display order.
func (s *Service) Groups() []settings.Group { return s.cat().Groups() }

func defaultJSON(d *settings.Def) json.RawMessage {
	b, _ := json.Marshal(d.Default)
	return b
}

func item(d *settings.Def, platform, tenant map[string]domain.Stored) settings.Item {
	it := settings.Item{Def: d, Value: defaultJSON(d), Source: settings.FromDefault}
	apply := func(st domain.Stored, src settings.Source) {
		it.Value, it.Source = st.Value, src
		by, at := st.UpdatedBy, st.UpdatedAt
		it.UpdatedBy, it.UpdatedAt = &by, &at
	}
	if st, ok := platform[d.Key]; ok {
		apply(st, settings.FromPlatform)
	}
	if d.Scope != settings.ScopePlatform {
		if st, ok := tenant[d.Key]; ok {
			apply(st, st.Source)
			it.Locked = st.Source == settings.FromOperator
		}
	}
	return it
}

func sorted(items []settings.Item, cat *settings.Catalog) []settings.Item {
	order := map[string]int{}
	for i, g := range cat.Groups() {
		order[g.ID] = i
	}
	sort.SliceStable(items, func(a, b int) bool { return order[items[a].Group] < order[items[b].Group] })
	return items
}

// TenantItems is what an organization sees: its own settings (tenant scope, editable unless locked) and the
// operator-managed ones (read-only). Platform-scope settings are not shown to organizations.
func (s *Service) TenantItems(ctx context.Context, tenantID string, canWrite bool) ([]settings.Item, error) {
	pv, err := s.platformValues(ctx)
	if err != nil {
		return nil, err
	}
	tv, err := s.tenantValues(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	out := []settings.Item{}
	for _, d := range s.cat().Defs(settings.ScopeTenant, settings.ScopeOperator) {
		it := item(d, pv, tv)
		it.Editable = canWrite && d.Scope == settings.ScopeTenant && !it.Locked
		out = append(out, it)
	}
	return sorted(out, s.cat()), nil
}

// ---- changes by an organization ---------------------------------------------------------------------------------

type InvalidError struct{ Problems map[string]string }

func (e InvalidError) Error() string   { return fmt.Sprintf("%d setting(s) rejected", len(e.Problems)) }
func (e InvalidError) Is(t error) bool { return t == settings.ErrInvalid }

// FieldProblems implements settings.FieldErrors.
func (e InvalidError) FieldProblems() map[string]string { return e.Problems }

// SetTenant changes organization-scope settings. Everything is validated first; one bad value rejects the whole call.
func (s *Service) SetTenant(ctx context.Context, tenantID, actor string, values map[string]json.RawMessage) error {
	return s.changeTenant(ctx, tenantID, actor, values, nil)
}

// ResetTenant removes the organization's own values so they inherit again.
func (s *Service) ResetTenant(ctx context.Context, tenantID, actor string, keys []string) error {
	return s.changeTenant(ctx, tenantID, actor, nil, keys)
}

func (s *Service) changeTenant(ctx context.Context, tenantID, actor string, values map[string]json.RawMessage, reset []string) error {
	tv, err := s.Repo.TenantValues(ctx, tenantID) // fresh read, not the cache: locks must be honoured exactly
	if err != nil {
		return err
	}
	problems := map[string]string{}
	var changes []domain.Change
	check := func(key string) (*settings.Def, bool) {
		d, ok := s.cat().Lookup(key)
		switch {
		case !ok:
			problems[key] = settings.ErrUnknownKey.Error()
		case d.Scope != settings.ScopeTenant:
			problems[key] = settings.ErrScope.Error()
		case tv[key].Source == settings.FromOperator:
			problems[key] = domain.ErrLocked.Error()
		default:
			return d, true
		}
		return nil, false
	}
	for key, raw := range values {
		d, ok := check(key)
		if !ok {
			continue
		}
		canon, _, err := d.Validate(raw)
		if err != nil {
			problems[key] = err.Error()
			continue
		}
		if cur, has := tv[key]; has && bytes.Equal(canonical(cur.Value), canon) {
			continue // unchanged
		}
		var old json.RawMessage
		if cur, has := tv[key]; has {
			old = cur.Value
		}
		changes = append(changes, domain.Change{Key: key, Old: old, New: canon})
	}
	for _, key := range reset {
		if _, ok := check(key); !ok {
			continue
		}
		if cur, has := tv[key]; has {
			changes = append(changes, domain.Change{Key: key, Old: cur.Value})
		}
	}
	if len(problems) > 0 {
		return InvalidError{Problems: problems}
	}
	slices.SortFunc(changes, func(a, b domain.Change) int { return compareStr(a.Key, b.Key) })
	if len(changes) == 0 {
		return nil
	}
	if err := s.Repo.ApplyTenant(ctx, tenantID, actor, changes); err != nil {
		return err
	}
	s.invalidate(tenantID)
	return nil
}

func compareStr(a, b string) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	}
	return 0
}

func canonical(b json.RawMessage) []byte {
	var v any
	if json.Unmarshal(b, &v) != nil {
		return b
	}
	out, _ := json.Marshal(v)
	return out
}

func (s *Service) TenantHistory(ctx context.Context, tenantID string, limit int) ([]settings.HistoryEntry, error) {
	return s.Repo.TenantHistory(ctx, tenantID, limit)
}

// ---- operator side ----------------------------------------------------------------------------------------------

// PlatformItems lists every setting with its platform-level value. For organization-scope settings the platform value
// is the default every organization inherits.
func (s *Service) PlatformItems(ctx context.Context) ([]settings.Item, error) {
	pv, err := s.Repo.PlatformValues(ctx)
	if err != nil {
		return nil, err
	}
	out := []settings.Item{}
	for _, d := range s.cat().Defs() {
		it := item(d, pv, nil)
		it.Editable = true
		out = append(out, it)
	}
	return sorted(out, s.cat()), nil
}

func (s *Service) validateAll(values map[string]json.RawMessage, keys []string) (map[string]json.RawMessage, error) {
	problems := map[string]string{}
	canon := map[string]json.RawMessage{}
	for key, raw := range values {
		d, ok := s.cat().Lookup(key)
		if !ok {
			problems[key] = settings.ErrUnknownKey.Error()
			continue
		}
		c, _, err := d.Validate(raw)
		if err != nil {
			problems[key] = err.Error()
			continue
		}
		canon[key] = c
	}
	for _, key := range keys {
		if _, ok := s.cat().Lookup(key); !ok {
			problems[key] = settings.ErrUnknownKey.Error()
		}
	}
	if len(problems) > 0 {
		return nil, InvalidError{Problems: problems}
	}
	return canon, nil
}

func (s *Service) SetPlatform(ctx context.Context, actor string, values map[string]json.RawMessage) error {
	canon, err := s.validateAll(values, nil)
	if err != nil {
		return err
	}
	for key, v := range canon {
		if err := s.Repo.OperatorSetPlatform(ctx, actor, key, v, false); err != nil {
			return err
		}
	}
	s.invalidate("")
	return nil
}

func (s *Service) ResetPlatform(ctx context.Context, actor string, keys []string) error {
	if _, err := s.validateAll(nil, keys); err != nil {
		return err
	}
	for _, key := range keys {
		if err := s.Repo.OperatorSetPlatform(ctx, actor, key, nil, true); err != nil {
			return err
		}
	}
	s.invalidate("")
	return nil
}

// OperatorTenantItems is the effective view of one organization, with what is locked by an operator.
func (s *Service) OperatorTenantItems(ctx context.Context, tenantID string) ([]settings.Item, error) {
	pv, err := s.Repo.PlatformValues(ctx)
	if err != nil {
		return nil, err
	}
	tv, err := s.Repo.OperatorTenantValues(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	out := []settings.Item{}
	for _, d := range s.cat().Defs(settings.ScopeTenant, settings.ScopeOperator) {
		it := item(d, pv, tv)
		it.Editable = true
		out = append(out, it)
	}
	return sorted(out, s.cat()), nil
}

// SetTenantOverride fixes values for one organization; the organization cannot change them afterwards.
func (s *Service) SetTenantOverride(ctx context.Context, actor, tenantID string, values map[string]json.RawMessage) error {
	canon, err := s.validateAll(values, nil)
	if err != nil {
		return err
	}
	problems := map[string]string{}
	for key := range canon {
		if d, _ := s.cat().Lookup(key); d.Scope == settings.ScopePlatform {
			problems[key] = "platform-wide settings cannot be set for one organization"
		}
	}
	if len(problems) > 0 {
		return InvalidError{Problems: problems}
	}
	for key, v := range canon {
		found, err := s.Repo.OperatorSetTenant(ctx, actor, tenantID, key, v, false)
		if err != nil {
			return err
		}
		if !found {
			return domain.ErrNotFound
		}
	}
	s.invalidate(tenantID)
	return nil
}

// ClearTenantOverride removes an operator override, which also unlocks the setting for the organization.
func (s *Service) ClearTenantOverride(ctx context.Context, actor, tenantID string, keys []string) error {
	if _, err := s.validateAll(nil, keys); err != nil {
		return err
	}
	for _, key := range keys {
		found, err := s.Repo.OperatorSetTenant(ctx, actor, tenantID, key, nil, true)
		if err != nil {
			return err
		}
		if !found {
			return domain.ErrNotFound
		}
	}
	s.invalidate(tenantID)
	return nil
}

// OperatorHistory returns platform-level changes (tenantID nil) or those of one organization.
func (s *Service) OperatorHistory(ctx context.Context, tenantID *string, limit int) ([]settings.HistoryEntry, error) {
	return s.Repo.OperatorHistory(ctx, tenantID, limit)
}
