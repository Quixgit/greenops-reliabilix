//go:build integration

package tests

import (
	"context"
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/quixgit/greenops-reliabilix/backend/internal/app"
)

type settingsHarness struct {
	t     *testing.T
	c     api
	owner string // token of the organization's owner
	admin string // token of a platform operator (admin)
	view  string // token of an operator with the viewer role
	org   string
}

// newSettingsHarness builds the whole API against the real database with one organization and two operators.
func newSettingsHarness(t *testing.T) *settingsHarness {
	t.Helper()
	ctx := context.Background()
	ownerDB, apiPool := pool(t, "TEST_OWNER_DSN"), pool(t, "TEST_API_DSN")
	cfg := devConfig(t.TempDir())
	apiC, err := app.Build(ctx, cfg, quietLog(), apiPool, &taskRec{})
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(app.NewRouter(cfg, quietLog(), apiC.Verifier, apiC.Resolver, apiPool.Ping, apiC.Modules()...))
	t.Cleanup(srv.Close)

	rid := fmt.Sprint(time.Now().UnixNano())
	h := &settingsHarness{t: t, c: api{t: t, srv: srv}}
	h.owner = fmt.Sprintf("dev:own-%s:-:-:own-%s@corp.example", rid, rid)
	h.admin = fmt.Sprintf("dev:opa-%s:-:-:opa-%s@platform.example", rid, rid)
	h.view = fmt.Sprintf("dev:opv-%s:-:-:opv-%s@platform.example", rid, rid)
	if _, err := ownerDB.Exec(ctx, `INSERT INTO platform.operators (auth_sub, role) VALUES ($1,'admin'), ($2,'viewer')`, "opa-"+rid, "opv-"+rid); err != nil {
		t.Fatal(err)
	}
	h.org = str(h.c.must("POST", "/api/v1/onboarding", h.owner, `{"organization_name":"Settings Org"}`, 201), "tenant_id")
	t.Cleanup(func() {
		_, _ = ownerDB.Exec(ctx, `DELETE FROM settings.platform_values`)
		_, _ = ownerDB.Exec(ctx, `DELETE FROM settings.tenant_values WHERE tenant_id = $1`, h.org)
		_, _ = ownerDB.Exec(ctx, `DELETE FROM settings.history WHERE tenant_id = $1 OR tenant_id IS NULL`, h.org)
		_, _ = ownerDB.Exec(ctx, `DELETE FROM platform.operator_access_log WHERE operator_sub IN ($1,$2)`, "opa-"+rid, "opv-"+rid)
		_, _ = ownerDB.Exec(ctx, `DELETE FROM platform.operators WHERE auth_sub IN ($1,$2)`, "opa-"+rid, "opv-"+rid)
	})
	return h
}

func (h *settingsHarness) item(m map[string]any, key string) map[string]any {
	h.t.Helper()
	for _, it := range m["items"].([]any) {
		if x := it.(map[string]any); str(x, "key") == key {
			return x
		}
	}
	h.t.Fatalf("setting %s not in the answer", key)
	return nil
}

func (h *settingsHarness) opSet(path, body string, want int) map[string]any {
	h.t.Helper()
	return h.c.must("PATCH", path, h.admin, body, want)
}

func TestSettingsInheritanceLockAndHistory(t *testing.T) {
	h := newSettingsHarness(t)
	const k = "security.invitation_expiry_days"
	own, opTenant := "/api/v1/settings", "/api/v1/operator/tenants/"+h.org+"/settings"

	v := h.c.must("GET", own, h.owner, "", 200)
	if it := h.item(v, k); it["source"] != "default" || it["editable"] != true || it["value"] != float64(7) {
		t.Fatalf("default: %v", it)
	}
	for _, it := range v["items"].([]any) {
		if sc := str(it.(map[string]any), "scope"); sc == "platform" {
			t.Fatalf("an organization must not see platform-scope settings: %v", it)
		}
	}

	// the platform sets the default for everybody; the organization still inherits it
	h.opSet("/api/v1/operator/settings", `{"values":{"`+k+`":10}}`, 200)
	if it := h.item(h.c.must("GET", own, h.owner, "", 200), k); it["source"] != "platform" || it["value"] != float64(10) {
		t.Fatalf("platform default: %v", it)
	}

	// the organization chooses its own value
	if it := h.item(h.c.must("PATCH", own, h.owner, `{"values":{"`+k+`":14}}`, 200), k); it["source"] != "tenant" || it["value"] != float64(14) {
		t.Fatalf("own value: %v", it)
	}

	// validation: one bad value rejects everything and names the key
	code, body, raw := h.c.do("PATCH", own, h.owner, `{"values":{"`+k+`":3,"security.api_keys_max_active":9999}}`)
	if code != 422 || sub(body, "errors")["security.api_keys_max_active"] == nil || sub(body, "errors")[k] != nil {
		t.Fatalf("validation answer = %d %s", code, raw)
	}
	if it := h.item(h.c.must("GET", own, h.owner, "", 200), k); it["value"] != float64(14) {
		t.Errorf("a rejected request must change nothing: %v", it)
	}
	// organization cannot reach operator/platform scope
	for _, key := range []string{"limits.projects", "platform.maintenance_enabled", "modules.reports", "nope.nothing"} {
		if code, _, _ := h.c.do("PATCH", own, h.owner, `{"values":{"`+key+`":1}}`); code != 422 {
			t.Errorf("%s: an organization changed a setting outside its scope (status %d)", key, code)
		}
	}

	// an operator fixes the value for this organization: locked
	h.opSet(opTenant, `{"values":{"`+k+`":5}}`, 200)
	it := h.item(h.c.must("GET", own, h.owner, "", 200), k)
	if it["source"] != "operator" || it["locked"] != true || it["editable"] != false || it["value"] != float64(5) {
		t.Fatalf("locked: %v", it)
	}
	if code, body, _ := h.c.do("PATCH", own, h.owner, `{"values":{"`+k+`":20}}`); code != 422 || !strings.Contains(fmt.Sprint(sub(body, "errors")[k]), "fixed by the platform") {
		t.Errorf("changing a locked setting: %d %v", code, body)
	}
	if code, _, _ := h.c.do("POST", own+"/reset", h.owner, `{"keys":["`+k+`"]}`); code != 422 {
		t.Errorf("resetting a locked setting: %d", code)
	}
	// unlock: it inherits again
	h.c.must("POST", opTenant+"/reset", h.admin, `{"keys":["`+k+`"]}`, 200)
	if it := h.item(h.c.must("GET", own, h.owner, "", 200), k); it["locked"] != false || it["source"] != "platform" {
		t.Errorf("after unlock: %v", it)
	}
	h.c.must("POST", own+"/reset", h.owner, `{"keys":["`+k+`"]}`, 200)
	if it := h.item(h.c.must("GET", own, h.owner, "", 200), k); it["source"] != "platform" || it["value"] != float64(10) {
		t.Errorf("after own reset: %v", it)
	}

	// history: both sides appear for the organization; platform changes do not
	hist := items(h.c.must("GET", own+"/history?limit=50", h.owner, "", 200))
	vias := map[string]int{}
	for _, e := range hist {
		m := e.(map[string]any)
		if m["tenant_id"] == nil {
			t.Errorf("platform-level history leaked to an organization: %v", m)
		}
		vias[str(m, "via")]++
	}
	if vias["tenant"] < 1 || vias["operator"] < 2 { // an operator override replaces the organization's own value, so unlocking leaves nothing for it to reset
		t.Errorf("history by source = %v", vias)
	}
	opHist := items(h.c.must("GET", "/api/v1/operator/settings/history", h.view, "", 200))
	if len(opHist) == 0 || opHist[0].(map[string]any)["tenant_id"] != nil {
		t.Errorf("operator platform history = %v", opHist)
	}

	// the operator's actions are in the organization's audit log
	found := 0
	for _, a := range items(h.c.must("GET", "/api/v1/audit-log?limit=100", h.owner, "", 200)) {
		if strings.HasPrefix(str(a.(map[string]any), "action"), "operator.setting_") {
			found++
		}
	}
	if found < 2 {
		t.Errorf("operator setting changes in the audit log: %d", found)
	}
}

func TestSettingsPermissions(t *testing.T) {
	h := newSettingsHarness(t)
	own := "/api/v1/settings"
	viewer := fmt.Sprintf("dev:v-%s:%s:viewer", h.org, h.org)
	engineer := fmt.Sprintf("dev:e-%s:%s:engineer", h.org, h.org)
	h.c.must("GET", own, viewer, "", 403)
	h.c.must("GET", own, engineer, "", 200)
	h.c.must("PATCH", own, engineer, `{"values":{"security.invitation_expiry_days":3}}`, 403)
	h.c.must("GET", own, "", "", 401)
	if it := h.item(h.c.must("GET", own, engineer, "", 200), "security.invitation_expiry_days"); it["editable"] != false {
		t.Errorf("a read-only role must see nothing editable: %v", it)
	}
	// operators: viewer reads, only admin writes; organizations' owners are not operators
	h.c.must("GET", "/api/v1/operator/settings", h.owner, "", 403)
	h.c.must("GET", "/api/v1/operator/settings", h.view, "", 200)
	h.c.must("PATCH", "/api/v1/operator/settings", h.view, `{"values":{"platform.signups_enabled":false}}`, 403)
	h.c.must("PATCH", "/api/v1/operator/tenants/"+h.org+"/settings", h.view, `{"values":{"limits.projects":1}}`, 403)
	h.c.must("PATCH", "/api/v1/operator/tenants/"+h.org+"/settings", h.admin, `{"values":{"platform.signups_enabled":false}}`, 422) // platform-wide only
	h.c.must("GET", "/api/v1/operator/tenants/00000000-0000-4000-8000-0000000000ff/settings", h.admin, "", 200)                     // unknown org: empty overrides, defaults
	h.c.must("PATCH", "/api/v1/operator/tenants/00000000-0000-4000-8000-0000000000ff/settings", h.admin, `{"values":{"limits.projects":1}}`, 404)
}

func TestModuleSwitchesMaintenanceAndSignup(t *testing.T) {
	h := newSettingsHarness(t)
	opTenant := "/api/v1/operator/tenants/" + h.org + "/settings"

	h.c.must("GET", "/api/v1/reports", h.owner, "", 200)
	h.opSet(opTenant, `{"values":{"modules.reports":false}}`, 200)
	code, body, _ := h.c.do("GET", "/api/v1/reports", h.owner, "")
	if code != 403 || str(body, "title") != "module disabled" {
		t.Fatalf("disabled module = %d %v", code, body)
	}
	h.c.must("POST", "/api/v1/reports", h.owner, `{"kind":"carbon","format":"csv","period_start":"2026-01-01","period_end":"2026-01-02"}`, 403)
	h.c.must("GET", "/api/v1/projects", h.owner, "", 200) // other modules are unaffected
	h.c.must("POST", opTenant+"/reset", h.admin, `{"keys":["modules.reports"]}`, 200)
	h.c.must("GET", "/api/v1/reports", h.owner, "", 200)

	// maintenance: reading works, changing does not, the operator console is exempt
	h.opSet("/api/v1/operator/settings", `{"values":{"platform.maintenance_enabled":true,"platform.maintenance_message":"Back at noon"}}`, 200)
	h.c.must("GET", "/api/v1/projects", h.owner, "", 200)
	code, body, _ = h.c.do("POST", "/api/v1/projects", h.owner, `{"name":"during","functional_unit":"request"}`)
	if code != 503 || str(body, "detail") != "Back at noon" {
		t.Fatalf("write during maintenance = %d %v", code, body)
	}
	h.c.must("GET", "/api/v1/operator/system", h.view, "", 200)
	h.c.must("POST", "/api/v1/operator/settings/reset", h.admin, `{"keys":["platform.maintenance_enabled","platform.maintenance_message"]}`, 200)
	h.c.must("POST", "/api/v1/projects", h.owner, `{"name":"after","functional_unit":"request"}`, 201)

	// sign-up policy
	newcomer := func(domain string) string {
		rid := fmt.Sprint(time.Now().UnixNano())
		return fmt.Sprintf("dev:nc-%s:-:-:nc-%s@%s", rid, rid, domain)
	}
	h.opSet("/api/v1/operator/settings", `{"values":{"platform.signups_enabled":false}}`, 200)
	if code, body, _ := h.c.do("POST", "/api/v1/onboarding", newcomer("corp.example"), `{"organization_name":"Nope"}`); code != 403 || str(body, "title") != "sign-up closed" {
		t.Errorf("closed sign-up = %d %v", code, body)
	}
	h.opSet("/api/v1/operator/settings", `{"values":{"platform.signups_enabled":true,"platform.signups_allowed_email_domains":["corp.example"]}}`, 200)
	if code, _, _ := h.c.do("POST", "/api/v1/onboarding", newcomer("other.example"), `{"organization_name":"Nope"}`); code != 403 {
		t.Errorf("a domain outside the allow-list got %d", code)
	}
	h.c.must("POST", "/api/v1/onboarding", newcomer("CORP.example"), `{"organization_name":"Welcome"}`, 201)
	// an existing member re-running onboarding is not a new sign-up problem for the allowed domain
	h.c.must("POST", "/api/v1/onboarding", h.owner, `{"organization_name":"Settings Org"}`, 201)
}

func TestLimitsAndPolicies(t *testing.T) {
	h := newSettingsHarness(t)
	opTenant := "/api/v1/operator/tenants/" + h.org + "/settings"

	// projects
	h.opSet(opTenant, `{"values":{"limits.projects":1}}`, 200)
	h.c.must("POST", "/api/v1/projects", h.owner, `{"name":"first","functional_unit":"request"}`, 201)
	code, body, _ := h.c.do("POST", "/api/v1/projects", h.owner, `{"name":"second","functional_unit":"request"}`)
	if code != 402 || !strings.Contains(str(body, "detail"), "limit of 1 projects") {
		t.Fatalf("project over the limit = %d %v", code, body)
	}
	h.c.must("GET", "/api/v1/projects", h.owner, "", 200) // existing data stays readable
	h.opSet(opTenant, `{"values":{"limits.projects":2}}`, 200)
	h.c.must("POST", "/api/v1/projects", h.owner, `{"name":"second","functional_unit":"request"}`, 201)
	h.opSet(opTenant, `{"values":{"limits.projects":0}}`, 200) // 0 = unlimited
	h.c.must("POST", "/api/v1/projects", h.owner, `{"name":"third","functional_unit":"request"}`, 201)

	// members (the owner counts, open invitations count)
	h.opSet(opTenant, `{"values":{"limits.members":2}}`, 200)
	h.c.must("POST", "/api/v1/invitations", h.owner, `{"email":"a@corp.example","role":"viewer"}`, 201)
	h.c.must("POST", "/api/v1/invitations", h.owner, `{"email":"b@corp.example","role":"viewer"}`, 402)

	// invitation e-mail domains and lifetime are the organization's own policy
	h.opSet(opTenant, `{"values":{"limits.members":0}}`, 200)
	h.c.must("PATCH", "/api/v1/settings", h.owner, `{"values":{"security.invitation_allowed_domains":["corp.example"],"security.invitation_expiry_days":2}}`, 200)
	if code, body, _ := h.c.do("POST", "/api/v1/invitations", h.owner, `{"email":"x@elsewhere.example","role":"viewer"}`); code != 422 || !strings.Contains(str(body, "detail"), "corp.example") {
		t.Errorf("invitation outside the allowed domains = %d %v", code, body)
	}
	inv := sub(h.c.must("POST", "/api/v1/invitations", h.owner, `{"email":"c@corp.example","role":"viewer"}`, 201), "invitation")
	exp, _ := time.Parse(time.RFC3339, str(inv, "expires_at"))
	if d := time.Until(exp); d < 47*time.Hour || d > 49*time.Hour {
		t.Errorf("invitation lifetime = %v, want about 48h", d)
	}

	// API keys
	h.c.must("PATCH", "/api/v1/settings", h.owner, `{"values":{"security.api_keys_max_active":1}}`, 200)
	k1 := sub(h.c.must("POST", "/api/v1/api-keys", h.owner, `{"name":"one","role":"ci"}`, 201), "api_key")
	if code, _, _ := h.c.do("POST", "/api/v1/api-keys", h.owner, `{"name":"two","role":"ci"}`); code != 422 {
		t.Errorf("second key over the limit = %d", code)
	}
	h.c.must("DELETE", "/api/v1/api-keys/"+str(k1, "id"), h.owner, "", 204)
	h.c.must("POST", "/api/v1/api-keys", h.owner, `{"name":"two","role":"ci"}`, 201) // a revoked key no longer counts

	// reports: allowed formats
	h.c.must("PATCH", "/api/v1/settings", h.owner, `{"values":{"reports.allowed_formats":["csv"]}}`, 200)
	period := `"period_start":"2026-01-01","period_end":"2026-01-02"`
	h.c.must("POST", "/api/v1/reports", h.owner, `{"kind":"carbon","format":"pdf",`+period+`}`, 403)
	h.c.must("POST", "/api/v1/reports", h.owner, `{"kind":"carbon","format":"csv",`+period+`}`, 202)

	// cloud providers and connection limit
	conn := func(provider, ref string) string {
		proj := str(items(h.c.must("GET", "/api/v1/projects", h.owner, "", 200))[0].(map[string]any), "id")
		return fmt.Sprintf(`{"project_id":"%s","provider":"%s","account_ref":"%s","credential_ref":"arn:aws:iam::123456789012:role/ReliabilixReadOnly"}`, proj, provider, ref)
	}
	h.opSet(opTenant, `{"values":{"connectors.allowed_providers":["gcp"]}}`, 200)
	if code, body, _ := h.c.do("POST", "/api/v1/cloud-accounts", h.owner, conn("aws", "123456789012")); code != 403 || str(body, "title") != "provider not enabled" {
		t.Errorf("a provider that is not allowed = %d %v", code, body)
	}
	h.opSet(opTenant, `{"values":{"connectors.allowed_providers":["aws"],"limits.cloud_connections":1}}`, 200)
	h.c.must("POST", "/api/v1/cloud-accounts", h.owner, conn("aws", "123456789012"), 201)
	h.c.must("POST", "/api/v1/cloud-accounts", h.owner, strings.ReplaceAll(conn("aws", "210987654321"), "123456789012", "210987654321"), 402)

	// notification destinations
	h.opSet(opTenant, `{"values":{"limits.notification_destinations":1}}`, 200)
	h.c.must("POST", "/api/v1/notifications/destinations", h.owner, `{"kind":"webhook","name":"a","target":"http://127.0.0.1:1/x"}`, 201)
	h.c.must("POST", "/api/v1/notifications/destinations", h.owner, `{"kind":"webhook","name":"b","target":"http://127.0.0.1:1/y"}`, 402)
}

func TestSettingsStorageRules(t *testing.T) {
	ctx := context.Background()
	h := newSettingsHarness(t)
	owner, apiPool, workerPool := pool(t, "TEST_OWNER_DSN"), pool(t, "TEST_API_DSN"), pool(t, "TEST_WORKER_DSN")
	h.opSet("/api/v1/operator/tenants/"+h.org+"/settings", `{"values":{"security.invitation_expiry_days":5}}`, 200)

	// an organization's own session can neither update nor delete an operator-locked row, nor create one
	tx, err := apiPool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, `SELECT set_config('app.tenant_id', $1, true)`, h.org); err != nil {
		t.Fatal(err)
	}
	tag, err := tx.Exec(ctx, `UPDATE settings.tenant_values SET value = '30' WHERE key = 'security.invitation_expiry_days'`)
	if err == nil && tag.RowsAffected() != 0 {
		t.Error("UPDATE of an operator-locked row succeeded")
	}
	_ = tx.Rollback(ctx)
	tx2, _ := apiPool.Begin(ctx)
	defer func() { _ = tx2.Rollback(ctx) }()
	_, _ = tx2.Exec(ctx, `SELECT set_config('app.tenant_id', $1, true)`, h.org)
	if tag, err := tx2.Exec(ctx, `DELETE FROM settings.tenant_values WHERE key = 'security.invitation_expiry_days'`); err == nil && tag.RowsAffected() != 0 {
		t.Error("DELETE of an operator-locked row succeeded")
	}
	_ = tx2.Rollback(ctx)
	tx3, _ := apiPool.Begin(ctx)
	defer func() { _ = tx3.Rollback(ctx) }()
	_, _ = tx3.Exec(ctx, `SELECT set_config('app.tenant_id', $1, true)`, h.org)
	if _, err := tx3.Exec(ctx, `INSERT INTO settings.tenant_values (tenant_id, key, value, source, updated_by) VALUES ($1,'limits.projects','999','operator','me')`, h.org); err == nil {
		t.Error("an organization session must not be able to write operator-sourced rows")
	}
	_ = tx3.Rollback(ctx)

	// roles: the worker reads but never writes; history is append-only; nobody but the owner role purges
	if _, err := workerPool.Exec(ctx, `SELECT count(*) FROM settings.platform_values`); err != nil {
		t.Errorf("the worker must read platform values: %v", err)
	}
	if _, err := workerPool.Exec(ctx, `INSERT INTO settings.platform_values (key, value, updated_by) VALUES ('x.y','1','w')`); err == nil {
		t.Error("the worker must not write platform values")
	}
	if _, err := apiPool.Exec(ctx, `INSERT INTO settings.platform_values (key, value, updated_by) VALUES ('x.y','1','a')`); err == nil {
		t.Error("the API role must not write platform values directly")
	}
	if _, err := apiPool.Exec(ctx, `DELETE FROM settings.history`); err == nil {
		t.Error("settings history must be append-only")
	}
	if _, err := apiPool.Exec(ctx, `SELECT settings.operator_set_platform('nobody', 'platform.signups_enabled', 'false'::jsonb, false)`); err == nil {
		t.Error("a non-operator must not be able to call the operator function")
	}
	if _, err := workerPool.Exec(ctx, `SELECT settings.operator_history(NULL, 10)`); err == nil {
		t.Error("the worker must not call operator functions")
	}
	// retention floors are enforced by the functions themselves
	if _, err := workerPool.Exec(ctx, `SELECT platform.purge_operator_access_log(interval '30 days')`); err == nil {
		t.Error("the operator access log must not be purgeable under 90 days")
	}
	_ = owner
}
