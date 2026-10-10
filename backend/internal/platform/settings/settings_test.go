package settings

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func mustDef(t *testing.T, key string) *Def {
	t.Helper()
	d, ok := Default().Lookup(key)
	if !ok {
		t.Fatalf("no setting %s", key)
	}
	return d
}

func TestCatalogIsConsistent(t *testing.T) {
	c := Default() // panics when a definition (or its default) is invalid
	if len(c.Defs()) < 25 {
		t.Errorf("catalog has %d settings", len(c.Defs()))
	}
	for _, d := range c.Defs() {
		if _, ok := c.Lookup(d.Key); !ok || d.Title == "" {
			t.Errorf("bad definition %s", d.Key)
		}
	}
	for _, s := range []Scope{ScopePlatform, ScopeTenant, ScopeOperator} {
		if len(c.Defs(s)) == 0 {
			t.Errorf("scope %s has no settings", s)
		}
	}
}

func TestNewCatalogRejectsBrokenDefinitions(t *testing.T) {
	g := []Group{{ID: "g", Title: "G"}}
	ok := Def{Key: "a.b", Group: "g", Title: "t", Description: "d", Scope: ScopeTenant, Type: Int, Default: int64(5), Min: f(1), Max: f(10)}
	if _, err := NewCatalog(g, []Def{ok}); err != nil {
		t.Fatal(err)
	}
	for name, mut := range map[string]func(*Def){
		"bad key":              func(d *Def) { d.Key = "Nope" },
		"single segment key":   func(d *Def) { d.Key = "nope" },
		"unknown group":        func(d *Def) { d.Group = "x" },
		"bad scope":            func(d *Def) { d.Scope = "world" },
		"no description":       func(d *Def) { d.Description = "" },
		"default out of range": func(d *Def) { d.Default = int64(99) },
		"default wrong type":   func(d *Def) { d.Default = "five" },
		"bad pattern":          func(d *Def) { d.Type, d.Default, d.Pattern = String, "x", "(" },
	} {
		d := ok
		mut(&d)
		if _, err := NewCatalog(g, []Def{d}); err == nil {
			t.Errorf("%s: catalog accepted a broken definition", name)
		}
	}
	if _, err := NewCatalog(g, []Def{ok, ok}); err == nil {
		t.Error("duplicate key accepted")
	}
}

func TestValidateByType(t *testing.T) {
	cases := []struct {
		key, in string
		ok      bool
		want    string
	}{
		{KeyInviteExpiryDays, `14`, true, `14`},
		{KeyInviteExpiryDays, `0`, false, ""},
		{KeyInviteExpiryDays, `31`, false, ""},
		{KeyInviteExpiryDays, `7.5`, false, ""},
		{KeyInviteExpiryDays, `"7"`, false, ""},
		{KeyInviteExpiryDays, `null`, false, ""},
		{KeyInviteExpiryDays, `7 8`, false, ""},
		{KeyMaintenanceEnabled, `true`, true, `true`},
		{KeyMaintenanceEnabled, `"true"`, false, ""},
		{KeyMaintenanceMessage, `"Back soon"`, true, `"Back soon"`},
		{KeyMaintenanceMessage, `"a\u0000b"`, false, ""},
		{KeyMaintenanceMessage, `1`, false, ""},
		{KeyReportFormats, `["csv","pdf"]`, true, `["csv","pdf"]`},
		{KeyReportFormats, `["csv","docx"]`, false, ""},
		{KeyReportFormats, `["csv","csv"]`, false, ""},
		{KeyReportFormats, `"csv"`, false, ""},
		{KeyInviteAllowedDomain, `["example.com","sub.example.org"]`, true, `["example.com","sub.example.org"]`},
		{KeyInviteAllowedDomain, `["Example.com"]`, false, ""},
		{KeyInviteAllowedDomain, `["not a domain"]`, false, ""},
		{KeyInviteAllowedDomain, `["a@b.com"]`, false, ""},
		{KeyInviteAllowedDomain, `[]`, true, `[]`},
		{KeyInviteAllowedDomain, `[""]`, false, ""},
		{KeyProvidersAllowed, `["aws"]`, true, `["aws"]`},
		{KeyProvidersAllowed, `["oracle"]`, false, ""},
	}
	for _, c := range cases {
		canon, _, err := mustDef(t, c.key).Validate(json.RawMessage(c.in))
		switch {
		case c.ok && err != nil:
			t.Errorf("%s %s: unexpected error %v", c.key, c.in, err)
		case !c.ok && !errors.Is(err, ErrInvalid):
			t.Errorf("%s %s: err = %v, want ErrInvalid", c.key, c.in, err)
		case c.ok && string(canon) != c.want:
			t.Errorf("%s %s: canonical %s, want %s", c.key, c.in, canon, c.want)
		}
	}
}

func TestValidateListBounds(t *testing.T) {
	d := mustDef(t, KeySignupsAllowedDomains)
	var items []string
	for i := 0; i < 101; i++ {
		items = append(items, "a"+strings.Repeat("b", i%5)+".com")
	}
	// duplicates would fail first; make them unique
	for i := range items {
		items[i] = "d" + strings.Repeat("x", i/10) + string(rune('a'+i%10)) + ".com"
	}
	raw, _ := json.Marshal(items)
	if _, _, err := d.Validate(raw); !errors.Is(err, ErrInvalid) {
		t.Errorf("a list over MaxItems must be refused, got %v", err)
	}
}

func TestDefaultsValidateAndTypeMatch(t *testing.T) {
	for _, d := range Default().Defs() {
		raw, _ := json.Marshal(d.Default)
		_, v, err := d.Validate(raw)
		if err != nil {
			t.Errorf("%s: default invalid: %v", d.Key, err)
			continue
		}
		switch d.Type {
		case Bool:
			if _, ok := v.(bool); !ok {
				t.Errorf("%s: bool default is %T", d.Key, v)
			}
		case Int:
			if _, ok := v.(int64); !ok {
				t.Errorf("%s: int default is %T", d.Key, v)
			}
		}
	}
}
