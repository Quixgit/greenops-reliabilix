package integrations

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/secretbox"
)

// ErrUnknown is returned for an integration or field that is not in the catalog.
var ErrUnknown = errors.New("unknown integration or field")

// ErrNoBox means secret values cannot be stored: no sealing key is configured.
var ErrNoBox = errors.New("integrations: no sealing key is configured, secrets cannot be stored")

// Source says where a value comes from.
type Source string

const (
	FromConsole     Source = "console"
	FromEnvironment Source = "environment"
	NotSet          Source = "not_set"
)

// Reader is what services depend on: the effective value of one field.
type Reader interface {
	Value(ctx context.Context, integration, key string) string
}

// Store reads values from the database (cached for a short time) with the environment as fallback.
type Store struct {
	Pool *pgxpool.Pool
	Box  *secretbox.Box
	// Env returns the value from the deployment environment for a field ("" when unset).
	Env func(integration, key string) string
	TTL time.Duration
	Now func() time.Time

	mu     sync.Mutex
	loaded time.Time
	rows   map[string]row
}

type row struct {
	value     string
	secret    bool
	updatedBy string
	updatedAt time.Time
}

func rk(integration, key string) string  { return integration + "/" + key }
func aad(integration, key string) string { return "integration:" + integration + ":" + key }

func (s *Store) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now()
}

func (s *Store) ttl() time.Duration {
	if s.TTL > 0 {
		return s.TTL
	}
	return 15 * time.Second
}

func (s *Store) load(ctx context.Context) (map[string]row, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.rows != nil && s.now().Sub(s.loaded) < s.ttl() {
		return s.rows, nil
	}
	if s.Pool == nil {
		s.rows, s.loaded = map[string]row{}, s.now()
		return s.rows, nil
	}
	rs, err := s.Pool.Query(ctx, `SELECT integration, key, value, secret, updated_by, updated_at FROM platform.integration_values`)
	if err != nil {
		if s.rows != nil {
			return s.rows, nil // keep serving the last good copy rather than dropping a key because of a hiccup
		}
		return nil, err
	}
	defer rs.Close()
	out := map[string]row{}
	for rs.Next() {
		var in, k string
		var r row
		if err := rs.Scan(&in, &k, &r.value, &r.secret, &r.updatedBy, &r.updatedAt); err != nil {
			return nil, err
		}
		if r.secret {
			if !s.Box.Configured() {
				continue
			}
			plain, err := s.Box.Open(r.value, aad(in, k))
			if err != nil {
				continue // cannot be opened (key rotated?): treated as not set, the console shows it as such
			}
			r.value = plain
		}
		out[rk(in, k)] = r
	}
	if err := rs.Err(); err != nil {
		return nil, err
	}
	s.rows, s.loaded = out, s.now()
	return out, nil
}

// Value returns the effective value: the console's, else the environment's, else "" (the field's default is for
// callers that need one: see ValueOrDefault).
func (s *Store) Value(ctx context.Context, integration, key string) string {
	v, _ := s.Resolve(ctx, integration, key)
	return v
}

// Resolve returns the effective value and where it came from.
func (s *Store) Resolve(ctx context.Context, integration, key string) (string, Source) {
	if rows, err := s.load(ctx); err == nil {
		if r, ok := rows[rk(integration, key)]; ok {
			return r.value, FromConsole
		}
	}
	if s.Env != nil {
		if v := s.Env(integration, key); v != "" {
			return v, FromEnvironment
		}
	}
	return "", NotSet
}

// ValueOrDefault is Value with the catalog default when nothing is set.
func (s *Store) ValueOrDefault(ctx context.Context, integration, key string) string {
	if v := s.Value(ctx, integration, key); v != "" {
		return v
	}
	if _, f, ok := Lookup(integration, key); ok {
		return f.Default
	}
	return ""
}

// FieldStatus describes one field for the console. A secret's value is never included.
type FieldStatus struct {
	Field
	Configured bool       `json:"configured"`
	Source     Source     `json:"source"`
	Value      string     `json:"value,omitempty"` // non-secret fields only
	Hint       string     `json:"hint,omitempty"`  // "••••abcd" for long secrets
	UpdatedAt  *time.Time `json:"updated_at,omitempty"`
	UpdatedBy  string     `json:"updated_by,omitempty"`
}

// Status is one integration with its fields and whether it is complete.
type Status struct {
	Integration
	Ready  bool          `json:"ready"`
	Fields []FieldStatus `json:"fields"`
}

func hint(secret string) string {
	if len(secret) < 12 {
		return "••••"
	}
	return "••••" + secret[len(secret)-4:]
}

// Statuses reports every catalog integration.
func (s *Store) Statuses(ctx context.Context) ([]Status, error) {
	rows, err := s.load(ctx)
	if err != nil {
		return nil, err
	}
	var out []Status
	for _, in := range Catalog() {
		st := Status{Integration: in, Ready: true}
		st.Fields = nil
		for _, f := range in.Fields {
			fs := FieldStatus{Field: f}
			if r, ok := rows[rk(in.ID, f.Key)]; ok {
				fs.Configured, fs.Source = true, FromConsole
				t, by := r.updatedAt, r.updatedBy
				fs.UpdatedAt, fs.UpdatedBy = &t, by
				if f.Secret {
					fs.Hint = hint(r.value)
				} else {
					fs.Value = r.value
				}
			} else if v := s.envValue(in.ID, f.Key); v != "" {
				fs.Configured, fs.Source = true, FromEnvironment
				if f.Secret {
					fs.Hint = hint(v)
				} else {
					fs.Value = v
				}
			} else {
				fs.Source = NotSet
			}
			if f.Required && !fs.Configured {
				st.Ready = false
			}
			st.Fields = append(st.Fields, fs)
		}
		out = append(out, st)
	}
	return out, nil
}

func (s *Store) envValue(integration, key string) string {
	if s.Env == nil {
		return ""
	}
	return s.Env(integration, key)
}

// Set stores or resets values. A nil pointer resets the field to the environment. All values are validated before
// anything is written, so a bad field leaves the integration as it was.
func (s *Store) Set(ctx context.Context, actor, integration string, values map[string]*string) error {
	if s.Pool == nil {
		return errors.New("integrations: no database")
	}
	if _, _, ok := Lookup(integration, ""); !ok {
		return ErrUnknown
	}
	type op struct {
		key, value string
		secret     bool
		reset      bool
	}
	var ops []op
	for k, v := range values {
		_, f, ok := Lookup(integration, k)
		if !ok {
			return fmt.Errorf("%w: %s", ErrUnknown, k)
		}
		if v == nil {
			ops = append(ops, op{key: k, reset: true})
			continue
		}
		if err := f.Validate(*v); err != nil {
			return fmt.Errorf("%s: %w", k, err)
		}
		o := op{key: k, value: *v, secret: f.Secret}
		if f.Secret {
			if !s.Box.Configured() {
				return ErrNoBox
			}
			sealed, err := s.Box.Seal(*v, aad(integration, k))
			if err != nil {
				return err
			}
			o.value = sealed
		}
		ops = append(ops, o)
	}
	for _, o := range ops {
		if _, err := s.Pool.Exec(ctx, `SELECT platform.operator_set_integration($1,$2,$3,$4,$5,$6)`, actor, integration, o.key, o.value, o.secret, o.reset); err != nil {
			return err
		}
	}
	s.mu.Lock()
	s.rows = nil // the next read sees the change at once on this process; others within the TTL
	s.mu.Unlock()
	return nil
}
