package http

import (
	"encoding/json"
	"testing"

	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/auth"
	"github.com/quixgit/greenops-reliabilix/backend/internal/tenants/domain"
)

func TestSCIMFilterEmail(t *testing.T) {
	for in, want := range map[string]string{`userName eq "a@b.co"`: "a@b.co", `USERNAME EQ "A@B.CO"`: "A@B.CO"} {
		if got, ok := scimFilterEmail(in); !ok || got != want {
			t.Errorf("%q: got %q %v", in, got, ok)
		}
	}
	for _, in := range []string{`displayName eq "x"`, `userName co "x"`, `userName eq`, ``} {
		if _, ok := scimFilterEmail(in); ok {
			t.Errorf("%q must not be accepted", in)
		}
	}
}

func TestSCIMApply(t *testing.T) {
	var in scimIn
	if err := json.Unmarshal([]byte(`{"userName":"Ann@Example.com","name":{"givenName":"Ann","familyName":"Lee"},"active":"False","roles":[{"value":"owner"},{"value":"Engineer"}]}`), &in); err != nil {
		t.Fatal(err)
	}
	u := in.apply(domain.SCIMUser{Active: true})
	if u.Email != "Ann@Example.com" || u.DisplayName != "Ann Lee" || u.Active || u.Role != auth.RoleEngineer {
		t.Fatalf("%+v", u) // owner is skipped, the next valid role wins; Azure's "False" string is understood
	}
	var none scimIn
	_ = json.Unmarshal([]byte(`{"userName":"x@y.zz"}`), &none)
	if r := none.apply(domain.SCIMUser{}).Role; r != auth.RoleViewer {
		t.Fatalf("default role %q", r)
	}
	var bad scimIn
	if json.Unmarshal([]byte(`{"active":"maybe"}`), &bad) == nil {
		t.Fatal("a non-boolean active must be refused")
	}
}

func TestSCIMPatchKey(t *testing.T) {
	if k := scimPatchKey("", json.RawMessage(`{"active":false}`)); string(k["active"]) != "false" {
		t.Fatal("Okta style, no path")
	}
	if k := scimPatchKey("active", json.RawMessage(`"False"`)); string(k["active"]) != `"False"` {
		t.Fatal("Azure style, path active")
	}
	if scimPatchKey("addresses", json.RawMessage(`[]`)) != nil {
		t.Fatal("unknown attributes are ignored")
	}
}
