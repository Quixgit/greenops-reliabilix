package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

type fakeResolver struct {
	tenant string
	role   Role
	err    error
	wanted string
}

func (f *fakeResolver) Resolve(_ context.Context, _ string, wanted string) (string, Role, error) {
	f.wanted = wanted
	return f.tenant, f.role, f.err
}

func call(mw func(http.Handler) http.Handler, token, tenantHeader string) (int, Claims) {
	var got Claims
	h := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { got, _ = FromContext(r.Context()) }))
	req := httptest.NewRequestWithContext(context.Background(), "GET", "/", nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if tenantHeader != "" {
		req.Header.Set("X-Tenant-ID", tenantHeader)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec.Code, got
}

func TestAuthenticateResolvesTenantFromDB(t *testing.T) {
	res := &fakeResolver{tenant: "T1", role: RoleEngineer}
	mw := Authenticate(DevVerifier{}, res)

	code, c := call(mw, "dev:alice:-:-:a@b.co", "T1")
	if code != 200 || c.TenantID != "T1" || c.Role != RoleEngineer || c.Email != "a@b.co" {
		t.Fatalf("resolved claims wrong: %d %+v", code, c)
	}
	if res.wanted != "T1" {
		t.Errorf("X-Tenant-ID not forwarded to resolver: %q", res.wanted)
	}
	// A token that already names a tenant (dev only) bypasses the resolver.
	if code, c := call(mw, "dev:bob:T9:viewer", ""); code != 200 || c.TenantID != "T9" {
		t.Errorf("explicit tenant: %d %+v", code, c)
	}
	if code, _ := call(mw, "", ""); code != 401 {
		t.Errorf("missing token = %d", code)
	}
	if code, _ := call(mw, "garbage", ""); code != 401 {
		t.Errorf("bad token = %d", code)
	}
	res.err = ErrNoMembership
	if code, _ := call(mw, "dev:carol:-:-", ""); code != 403 {
		t.Errorf("no membership = %d, want 403", code)
	}
}

func TestAuthenticateIdentityRejectsAPIKeys(t *testing.T) {
	v := KeyedVerifier{Fallback: DevVerifier{}, Lookup: func(context.Context, string) (string, Role, string, error) {
		return "T1", RoleCI, "k1", nil
	}}
	if code, _ := call(AuthenticateIdentity(v), APIKeyPrefix+"abc", ""); code != 401 {
		t.Errorf("api key accepted on identity-only route: %d", code)
	}
	if code, c := call(AuthenticateIdentity(v), "dev:dan:-:-", ""); code != 200 || c.Subject != "dan" {
		t.Errorf("identity route: %d %+v", code, c)
	}
}

func TestKeyedVerifier(t *testing.T) {
	key, prefix, err := GenerateAPIKey()
	if err != nil || len(prefix) != len(APIKeyPrefix)+6 {
		t.Fatalf("generate: %v %q", err, prefix)
	}
	want := HashAPIKey(key)
	v := KeyedVerifier{Fallback: DevVerifier{}, Lookup: func(_ context.Context, h string) (string, Role, string, error) {
		if h != want {
			return "", "", "", ErrUnauthenticated
		}
		return "T1", RoleCI, "k1", nil
	}}
	c, err := v.Verify(context.Background(), key)
	if err != nil || c.TenantID != "T1" || c.Role != RoleCI || c.KeyID != "k1" {
		t.Fatalf("valid key: %v %+v", err, c)
	}
	if _, err := v.Verify(context.Background(), key+"x"); err == nil {
		t.Error("wrong key accepted")
	}
	// RBAC: a CI key may evaluate but nothing else.
	if !c.Role.Can(PermCIEvaluate) || c.Role.Can(PermProjectRead) {
		t.Error("ci role permissions wrong")
	}
}

func TestRequire(t *testing.T) {
	h := Require(PermProjectWrite)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	for role, want := range map[Role]int{RoleAdmin: 200, RoleViewer: 403} {
		req := httptest.NewRequestWithContext(WithClaims(context.Background(), Claims{TenantID: "T", Role: role}), "GET", "/", nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != want {
			t.Errorf("%s = %d, want %d", role, rec.Code, want)
		}
	}
}
