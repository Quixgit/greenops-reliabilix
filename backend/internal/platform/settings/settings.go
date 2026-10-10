// Package settings is the platform's configuration engine (ADR-0023). Every adjustable behaviour is one Def in the
// catalog: its key, type, bounds, default, who may change it and how it is described. Storage, validation, history,
// the API and the admin screens are all generated from the catalog, so adding a setting means adding a Def and the
// code that honours it, never a new table, endpoint or form.
//
// Three scopes decide who may change a value:
//   - Platform: the whole deployment; operators only (maintenance mode, retention, sign-ups ...).
//   - Tenant:   an organization's own choices; its owners and admins may change them, and the platform supplies the
//     default every organization inherits. An operator can override a value for one organization and lock it.
//   - Operator: per organization, operator only; the organization sees it read-only (limits, enabled modules ...).
//
// Effective value of a key for an organization: the organization's own value, else the platform value, else the default.
// Secrets never live here: they stay in the environment / secret manager (ADR-0010).
package settings

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"regexp"
	"slices"
	"strings"
)

type Scope string

const (
	ScopePlatform Scope = "platform"
	ScopeTenant   Scope = "tenant"
	ScopeOperator Scope = "operator"
)

type Type string

const (
	Bool   Type = "bool"
	Int    Type = "int"
	Float  Type = "float"
	String Type = "string"
	Enum   Type = "enum"
	List   Type = "list" // list of strings
)

// Def describes one setting.
type Def struct {
	Key         string `json:"key"`
	Group       string `json:"group"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Scope       Scope  `json:"scope"`
	Type        Type   `json:"type"`
	Default     any    `json:"default"` // bool, int64, float64, string or []string, matching Type
	Unit        string `json:"unit,omitempty"`

	Min      *float64 `json:"min,omitempty"` // Int, Float
	Max      *float64 `json:"max,omitempty"`
	Enum     []string `json:"enum,omitempty"`      // Enum values
	ListEnum []string `json:"list_enum,omitempty"` // List: allowed items (empty = free text matching Pattern)
	MaxItems int      `json:"max_items,omitempty"` // List
	MinItems int      `json:"min_items,omitempty"` // List
	MaxLen   int      `json:"max_len,omitempty"`   // String, list items
	Pattern  string   `json:"pattern,omitempty"`   // String, list items

	// Note is shown next to the setting: how it is applied, what it does not do.
	Note string `json:"note,omitempty"`

	re *regexp.Regexp
}

// Group orders and describes settings in the UI.
type Group struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
}

// FieldErrors is implemented by errors that carry one message per rejected setting; HTTP layers turn them into a 422
// that names the fields.
type FieldErrors interface {
	error
	FieldProblems() map[string]string
}

var (
	ErrNotFound   = errors.New("not found")
	ErrUnknownKey = errors.New("unknown setting")
	ErrInvalid    = errors.New("invalid setting value")
	ErrScope      = errors.New("this setting cannot be changed here")
)

func f(v float64) *float64 { return &v }

// Validate checks raw (JSON) against the definition and returns the canonical JSON to store plus the typed value.
func (d *Def) Validate(raw json.RawMessage) (json.RawMessage, any, error) {
	bad := func(format string, a ...any) (json.RawMessage, any, error) {
		return nil, nil, fmt.Errorf("%w: %s: %s", ErrInvalid, d.Key, fmt.Sprintf(format, a...))
	}
	dec := json.NewDecoder(strings.NewReader(string(raw)))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil || dec.More() {
		return bad("not valid JSON")
	}
	var out any
	switch d.Type {
	case Bool:
		b, ok := v.(bool)
		if !ok {
			return bad("must be true or false")
		}
		out = b
	case Int, Float:
		n, ok := v.(json.Number)
		if !ok {
			return bad("must be a number")
		}
		var x float64
		if d.Type == Int {
			i, err := n.Int64()
			if err != nil {
				return bad("must be a whole number")
			}
			x, out = float64(i), i
		} else {
			fl, err := n.Float64()
			if err != nil || math.IsNaN(fl) || math.IsInf(fl, 0) {
				return bad("must be a number")
			}
			x, out = fl, fl
		}
		if d.Min != nil && x < *d.Min || d.Max != nil && x > *d.Max {
			return bad("must be between %v and %v%s", derefOr(d.Min, "-inf"), derefOr(d.Max, "+inf"), unitSuffix(d.Unit))
		}
	case String:
		s, ok := v.(string)
		if !ok {
			return bad("must be text")
		}
		if err := d.checkText(s); err != nil {
			return bad("%v", err)
		}
		out = s
	case Enum:
		s, ok := v.(string)
		if !ok || !slices.Contains(d.Enum, s) {
			return bad("must be one of: %s", strings.Join(d.Enum, ", "))
		}
		out = s
	case List:
		arr, ok := v.([]any)
		if !ok {
			return bad("must be a list")
		}
		if d.MaxItems > 0 && len(arr) > d.MaxItems {
			return bad("at most %d items", d.MaxItems)
		}
		if len(arr) < d.MinItems {
			return bad("at least %d item(s) required", d.MinItems)
		}
		items := make([]string, 0, len(arr))
		for _, e := range arr {
			s, ok := e.(string)
			if !ok {
				return bad("items must be text")
			}
			s = strings.TrimSpace(s)
			if len(d.ListEnum) > 0 && !slices.Contains(d.ListEnum, s) {
				return bad("%q is not allowed (allowed: %s)", s, strings.Join(d.ListEnum, ", "))
			}
			if len(d.ListEnum) == 0 {
				if err := d.checkText(s); err != nil {
					return bad("%v", err)
				}
			}
			if slices.Contains(items, s) {
				return bad("duplicate item %q", s)
			}
			items = append(items, s)
		}
		out = items
	default:
		return bad("unsupported type")
	}
	canon, err := json.Marshal(out)
	if err != nil {
		return nil, nil, err
	}
	return canon, out, nil
}

func (d *Def) checkText(s string) error {
	if s == "" && d.Type == String {
		return nil
	}
	if s == "" {
		return errors.New("must not be empty")
	}
	if strings.ContainsFunc(s, func(r rune) bool { return r < 0x20 || r == 0x7f }) {
		return errors.New("must not contain control characters")
	}
	max := d.MaxLen
	if max == 0 {
		max = 200
	}
	if len(s) > max {
		return fmt.Errorf("at most %d characters", max)
	}
	if d.re != nil && !d.re.MatchString(s) {
		return errors.New("has an invalid format")
	}
	return nil
}

func derefOr(p *float64, def string) string {
	if p == nil {
		return def
	}
	return fmt.Sprint(*p)
}

func unitSuffix(u string) string {
	if u == "" {
		return ""
	}
	return " " + u
}

// Catalog is the immutable set of definitions.
type Catalog struct {
	defs   []*Def
	byKey  map[string]*Def
	groups []Group
}

// NewCatalog validates the definitions themselves (a broken catalog fails at startup, not in production).
func NewCatalog(groups []Group, defs []Def) (*Catalog, error) {
	c := &Catalog{byKey: map[string]*Def{}, groups: groups}
	gid := map[string]bool{}
	for _, g := range groups {
		gid[g.ID] = true
	}
	for i := range defs {
		d := defs[i]
		switch {
		case !keyRe.MatchString(d.Key):
			return nil, fmt.Errorf("settings: bad key %q", d.Key)
		case c.byKey[d.Key] != nil:
			return nil, fmt.Errorf("settings: duplicate key %q", d.Key)
		case !gid[d.Group]:
			return nil, fmt.Errorf("settings: %s: unknown group %q", d.Key, d.Group)
		case d.Scope != ScopePlatform && d.Scope != ScopeTenant && d.Scope != ScopeOperator:
			return nil, fmt.Errorf("settings: %s: bad scope", d.Key)
		case d.Title == "" || d.Description == "":
			return nil, fmt.Errorf("settings: %s: title and description are required", d.Key)
		}
		if d.Pattern != "" {
			re, err := regexp.Compile(d.Pattern)
			if err != nil {
				return nil, fmt.Errorf("settings: %s: %w", d.Key, err)
			}
			d.re = re
		}
		raw, err := json.Marshal(d.Default)
		if err != nil {
			return nil, fmt.Errorf("settings: %s: %w", d.Key, err)
		}
		if _, _, err := d.Validate(raw); err != nil {
			return nil, fmt.Errorf("settings: default of %w", err)
		}
		c.byKey[d.Key] = &d
		c.defs = append(c.defs, &d)
	}
	return c, nil
}

var keyRe = regexp.MustCompile(`^[a-z][a-z0-9_]*(\.[a-z][a-z0-9_]*)+$`)

func (c *Catalog) Lookup(key string) (*Def, bool) { d, ok := c.byKey[key]; return d, ok }
func (c *Catalog) Groups() []Group                { return c.groups }

// Defs returns the definitions visible for the given scopes, in catalog order.
func (c *Catalog) Defs(scopes ...Scope) []*Def {
	out := []*Def{}
	for _, d := range c.defs {
		if len(scopes) == 0 || slices.Contains(scopes, d.Scope) {
			out = append(out, d)
		}
	}
	return out
}
