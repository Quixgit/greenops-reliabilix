package settings

import (
	"context"
	"encoding/json"
	"time"
)

// Source says where an effective value comes from.
type Source string

const (
	FromDefault  Source = "default"  // the catalog default
	FromPlatform Source = "platform" // set by operators for everybody
	FromTenant   Source = "tenant"   // chosen by the organization
	FromOperator Source = "operator" // set by an operator for this organization (locked)
)

// Item is a definition together with its effective value, ready for the API.
type Item struct {
	*Def
	Value     json.RawMessage `json:"value"`
	Source    Source          `json:"source"`
	Locked    bool            `json:"locked"`   // an operator fixed the value for this organization
	Editable  bool            `json:"editable"` // the caller may change it right here
	UpdatedBy *string         `json:"updated_by"`
	UpdatedAt *time.Time      `json:"updated_at"`
}

// HistoryEntry is one change. Values are the JSON that was stored.
type HistoryEntry struct {
	At       time.Time       `json:"at"`
	TenantID *string         `json:"tenant_id"`
	Key      string          `json:"key"`
	Old      json.RawMessage `json:"old_value"`
	New      json.RawMessage `json:"new_value"`
	Actor    string          `json:"actor"`
	Via      string          `json:"via"`
}

// Reader is what domains use to honour a setting. Platform-scope keys ignore tenantID. A read that fails returns the
// catalog default: configuration must never take a request down, and defaults are always safe values.
type Reader interface {
	Bool(ctx context.Context, tenantID, key string) bool
	Int(ctx context.Context, tenantID, key string) int64
	String(ctx context.Context, tenantID, key string) string
	Strings(ctx context.Context, tenantID, key string) []string
}

// Defaults reads the catalog defaults only (tests, and modules wired without the settings engine).
type Defaults struct{}

func (Defaults) def(key string) any {
	if d, ok := Default().Lookup(key); ok {
		return d.Default
	}
	panic("settings: unknown key " + key) // a typo in code, caught by the first test that touches it
}
func (d Defaults) Bool(_ context.Context, _, key string) bool { v, _ := d.def(key).(bool); return v }
func (d Defaults) Int(_ context.Context, _, key string) int64 { v, _ := d.def(key).(int64); return v }
func (d Defaults) String(_ context.Context, _, key string) string {
	v, _ := d.def(key).(string)
	return v
}
func (d Defaults) Strings(_ context.Context, _, key string) []string {
	v, _ := d.def(key).([]string)
	return append([]string(nil), v...)
}

// Fixed is a Reader with explicit values over the defaults (tests).
type Fixed map[string]any

func (f Fixed) pick(key string) any {
	if v, ok := f[key]; ok {
		return v
	}
	return Defaults{}.def(key)
}
func (f Fixed) Bool(_ context.Context, _, key string) bool { v, _ := f.pick(key).(bool); return v }
func (f Fixed) String(_ context.Context, _, key string) string {
	v, _ := f.pick(key).(string)
	return v
}
func (f Fixed) Int(_ context.Context, _, key string) int64 {
	switch v := f.pick(key).(type) {
	case int64:
		return v
	case int:
		return int64(v)
	}
	return 0
}
func (f Fixed) Strings(_ context.Context, _, key string) []string {
	v, _ := f.pick(key).([]string)
	return append([]string(nil), v...)
}
