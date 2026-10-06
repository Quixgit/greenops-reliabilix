//go:build integration

package tests

import (
	"context"
	"fmt"
	"math"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/hibiken/asynq"

	"github.com/quixgit/greenops-reliabilix/backend/internal/app"
	clouddomain "github.com/quixgit/greenops-reliabilix/backend/internal/cloudaccounts/domain"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/focus"
)

// fakeAWS stands in for the real AWS provider: it returns a Cost Explorer style payload.
type fakeAWS struct{ payload []byte }

func (fakeAWS) Provider() clouddomain.ProviderType                     { return clouddomain.AWS }
func (fakeAWS) Validate(context.Context, clouddomain.Connection) error { return nil }
func (f fakeAWS) GetUsage(context.Context, clouddomain.UsageRequest) ([]focus.RawBatch, error) {
	return []focus.RawBatch{{Provider: "aws", Source: "cost_explorer", Payload: f.payload}}, nil
}

// cePayload builds three final days of cost: EC2 $100 and S3 $20 in us-east-1, Lambda $5 in eu-central-1,
// Route 53 $1 global (no grid to attribute carbon to).
func cePayload(days []time.Time, use1, euc1 string) []byte {
	var parts []string
	for _, d := range days {
		parts = append(parts, fmt.Sprintf(`{"TimePeriod":{"Start":"%[1]s","End":"%[2]s"},"Estimated":false,"Groups":[
		 {"Keys":["Amazon Elastic Compute Cloud - Compute","%[3]s"],"Metrics":{"UnblendedCost":{"Amount":"100","Unit":"USD"},"AmortizedCost":{"Amount":"100","Unit":"USD"}}},
		 {"Keys":["Amazon Simple Storage Service","%[3]s"],"Metrics":{"UnblendedCost":{"Amount":"20","Unit":"USD"},"AmortizedCost":{"Amount":"20","Unit":"USD"}}},
		 {"Keys":["AWS Lambda","%[4]s"],"Metrics":{"UnblendedCost":{"Amount":"5","Unit":"USD"},"AmortizedCost":{"Amount":"5","Unit":"USD"}}},
		 {"Keys":["Amazon Route 53","NoRegion"],"Metrics":{"UnblendedCost":{"Amount":"1","Unit":"USD"},"AmortizedCost":{"Amount":"1","Unit":"USD"}}}]}`,
			d.Format(time.DateOnly), d.AddDate(0, 0, 1).Format(time.DateOnly), use1, euc1))
	}
	return []byte(`{"ResultsByTime":[` + strings.Join(parts, ",") + `]}`)
}

func TestEndToEndPipelineAndAPI(t *testing.T) {
	ctx := context.Background()
	owner, apiPool, workerPool := pool(t, "TEST_OWNER_DSN"), pool(t, "TEST_API_DSN"), pool(t, "TEST_WORKER_DSN")

	// Region ids are unique per run: grid intensity is global reference data shared by every test and by the
	// dev demo seed, and "latest reading before the end of the day" must not pick up someone else's readings.
	suffix := fmt.Sprint(time.Now().UnixNano() % 1_000_000_000)
	use1, usw2, euc1, eun1 := "use1-"+suffix, "usw2-"+suffix, "euc1-"+suffix, "eun1-"+suffix
	today := time.Now().UTC().Truncate(24 * time.Hour)
	days := []time.Time{today.AddDate(0, 0, -3), today.AddDate(0, 0, -2), today.AddDate(0, 0, -1)}
	reg := clouddomain.NewRegistry(fakeAWS{payload: cePayload(days, use1, euc1)})

	cfg := devConfig(t.TempDir())
	cfg.PlatformAWSAccountID = "111122223333"
	recA, recW := &taskRec{}, &taskRec{}
	apiC, err := app.Build(ctx, cfg, quietLog(), apiPool, recA, app.WithProviders(reg))
	if err != nil {
		t.Fatal(err)
	}
	workerC, err := app.Build(ctx, cfg, quietLog(), workerPool, recW, app.WithProviders(reg))
	if err != nil {
		t.Fatal(err)
	}
	mux := asynq.NewServeMux()
	for _, j := range workerC.JobRegistrars() {
		j.RegisterJobs(mux)
	}
	srv := httptest.NewServer(app.NewRouter(cfg, quietLog(), apiC.Verifier, apiC.Resolver, apiPool.Ping, apiC.Modules()...))
	defer srv.Close()
	c := api{t: t, srv: srv}

	// reference grid data (global): us-west-2 and eu-north-1 are cleaner than us-east-1
	for region, g := range map[string]float64{use1: 380, usw2: 150, euc1: 340, eun1: 28} {
		if _, err := owner.Exec(ctx, `INSERT INTO carbon.grid_intensity (provider, region_id, ts, g_co2e_per_kwh) VALUES ('electricitymaps',$1,now() - interval '1 hour',$2) ON CONFLICT DO NOTHING`, region, g); err != nil {
			t.Fatal(err)
		}
	}

	// identities are unique per run: the database is shared between test runs
	rid := fmt.Sprint(time.Now().UnixNano())
	person := func(name string) string { return fmt.Sprintf("dev:%s-%s:-:-:%s-%s@example.com", name, rid, name, rid) }
	alice, bob, carol, eve := person("alice"), person("bob"), person("carol"), person("eve")
	bobEmail := "bob-" + rid + "@example.com"

	// ---- onboarding: a brand-new identity has no tenant until it onboards ----
	code, body, _ := c.do("GET", "/api/v1/projects", alice, "")
	if code != 403 || str(body, "detail") != "onboarding_required" {
		t.Fatalf("pre-onboarding /projects = %d %v, want 403 onboarding_required", code, body)
	}
	tenant := str(c.must("POST", "/api/v1/onboarding", alice, `{"organization_name":"Acme"}`, 201), "tenant_id")
	if again := str(c.must("POST", "/api/v1/onboarding", alice, `{"organization_name":"Acme 2"}`, 201), "tenant_id"); again != tenant {
		t.Fatalf("onboarding is not idempotent: %s vs %s", again, tenant)
	}
	if me := c.must("GET", "/api/v1/me", alice, "", 200); str(me, "role") != "owner" || str(me, "tenant_id") != tenant {
		t.Fatalf("/me = %v", me)
	}
	if o := c.must("GET", "/api/v1/tenant", alice, "", 200); str(o, "name") != "Acme" {
		t.Fatalf("/tenant = %v", o)
	}
	c.must("POST", "/api/v1/onboarding", alice, `{"organization_name":""}`, 422)

	// ---- project, SCI denominator, residency policy ----
	proj := str(c.must("POST", "/api/v1/projects", alice, `{"name":"prod","functional_unit":"request"}`, 201), "id")
	c.must("POST", "/api/v1/projects", alice, `{"name":"prod","functional_unit":"request"}`, 409)
	c.must("POST", "/api/v1/projects/"+proj+"/functional-units", alice,
		fmt.Sprintf(`{"period_start":"%s","period_end":"%s","units":11000}`, today.AddDate(0, 0, -10).Format(time.DateOnly), today.Format(time.DateOnly)), 200)
	c.must("POST", "/api/v1/projects/"+proj+"/functional-units", alice, `{"period_start":"2026-01-02","period_end":"2026-01-01","units":5}`, 422)
	c.must("PUT", "/api/v1/projects/"+proj+"/policy", alice, fmt.Sprintf(`{"allowed_regions":["%s","%s","%s"],"ci_carbon_warn_kg":5}`, usw2, use1, euc1), 200)
	c.must("PUT", "/api/v1/projects/"+proj+"/policy", alice, `{"allowed_regions":["bad region!"]}`, 422)

	// ---- cloud account: pending -> external id -> verify -> sync ----
	c.must("POST", "/api/v1/cloud-accounts", alice, fmt.Sprintf(`{"project_id":"%s","provider":"azure","account_ref":"x","credential_ref":"y"}`, proj), 422)
	c.must("POST", "/api/v1/cloud-accounts", alice, fmt.Sprintf(`{"project_id":"%s","provider":"aws","account_ref":"123456789012","credential_ref":"AKIAIOSFODNN7EXAMPLE"}`, proj), 422)
	created := c.must("POST", "/api/v1/cloud-accounts", alice,
		fmt.Sprintf(`{"project_id":"%s","provider":"aws","account_ref":"123456789012","credential_ref":"arn:aws:iam::123456789012:role/ReliabilixReadOnly"}`, proj), 201)
	conn, setup := sub(created, "connection"), sub(created, "setup")
	connID := str(conn, "id")
	if str(conn, "sync_status") != "pending" || !strings.HasPrefix(str(setup, "external_id"), "rlx-") || setup["trust_policy"] == nil {
		t.Fatalf("connect response: %v", created)
	}
	c.must("POST", "/api/v1/cloud-accounts", alice,
		fmt.Sprintf(`{"project_id":"%s","provider":"aws","account_ref":"123456789012","credential_ref":"arn:aws:iam::123456789012:role/ReliabilixOther"}`, proj), 409)
	// ---- FOCUS export location: validated, audited, reversible ----
	c.must("PUT", "/api/v1/cloud-accounts/"+connID+"/export", alice, `{"bucket":"acme-billing","prefix":"../x","name":"rlx","region":"eu-central-1"}`, 422)
	c.must("PUT", "/api/v1/cloud-accounts/"+connID+"/export", alice, `{"bucket":"acme-billing","name":"rlx","region":"https://evil"}`, 422)
	withExport := c.must("PUT", "/api/v1/cloud-accounts/"+connID+"/export", alice, `{"bucket":"acme-billing","prefix":"exports","name":"rlx","region":"eu-central-1"}`, 200)
	if e := sub(withExport, "export"); str(e, "bucket") != "acme-billing" || str(withExport, "sync_status") != "pending" {
		t.Fatalf("export not stored: %v", withExport)
	}
	if cleared := c.must("DELETE", "/api/v1/cloud-accounts/"+connID+"/export", alice, "", 200); cleared["export"] != nil {
		t.Fatalf("export not cleared: %v", cleared)
	}

	if v := c.must("POST", "/api/v1/cloud-accounts/"+connID+"/verify", alice, "", 200); str(v, "sync_status") != "healthy" {
		t.Fatalf("verify: %v", v)
	}
	ran := pump(t, mux, recA, recW) // verify queued a sync; the chain sync -> carbon -> recommendations follows
	if strings.Join(ran, ",") != "cloudaccounts:sync_aws_account,carbon:calculate,recommendations:calculate" {
		t.Fatalf("job chain = %v", ran)
	}

	count := func(q string, args ...any) (n int) {
		if err := owner.QueryRow(ctx, q, args...).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return
	}
	if n := count(`SELECT count(*) FROM usage.usage_records WHERE tenant_id = $1`, tenant); n != 12 {
		t.Fatalf("usage rows = %d, want 12 (4 groups x 3 days)", n)
	}
	// FOCUS-shaped rows over the API
	u := c.must("GET", "/api/v1/usage?limit=100", alice, "", 200)
	if len(items(u)) != 12 {
		t.Fatalf("/usage items = %d", len(items(u)))
	}
	first := items(u)[0].(map[string]any)
	for _, k := range []string{"service_name", "service_category", "region_id", "billed_cost", "effective_cost", "currency", "charge_period_start", "charge_period_end"} {
		if _, ok := first[k]; !ok {
			t.Errorf("usage row lacks FOCUS field %s: %v", k, first)
		}
	}
	got := c.must("GET", "/api/v1/cloud-accounts/"+connID, alice, "", 200)
	if sub(got, "connection")["synced_through"] == nil || str(sub(got, "connection"), "sync_status") != "healthy" {
		t.Fatalf("connection after sync: %v", got)
	}
	runs := items(c.must("GET", "/api/v1/cloud-accounts/"+connID+"/sync-runs", alice, "", 200))
	if len(runs) != 1 || runs[0].(map[string]any)["status"] != "succeeded" || runs[0].(map[string]any)["records"].(float64) != 12 {
		t.Fatalf("sync runs: %v", runs)
	}

	// idempotent re-sync: same charge lines are replaced, never duplicated
	c.must("POST", "/api/v1/cloud-accounts/"+connID+"/sync", alice, "", 202)
	pump(t, mux, recA, recW)
	if n := count(`SELECT count(*) FROM usage.usage_records WHERE tenant_id = $1`, tenant); n != 12 {
		t.Fatalf("usage rows after re-sync = %d, want 12", n)
	}
	if n := count(`SELECT count(*) FROM carbon.calculations WHERE tenant_id = $1`, tenant); n != 9 {
		t.Fatalf("carbon rows = %d, want 9 (3 workloads x 3 days; the global Route 53 line is skipped)", n)
	}

	// ---- carbon: cost-based estimate, SCI from reported functional units ----
	sum := c.must("GET", "/api/v1/carbon/summary?project_id="+proj, alice, "", 200)
	// per day: EC2 100*.9*1.135*380/1000 + S3 20*.35*1.135*380/1000 + Lambda 5*.9*1.135*340/1000
	wantKg := 3 * (100*0.9*1.135*380 + 20*0.35*1.135*380 + 5*0.9*1.135*340) / 1000
	if sum["has_data"] != true || math.Abs(sum["carbon_kg_co2e"].(float64)-wantKg) > 0.05 {
		t.Fatalf("carbon summary = %v, want ~%.2f kg", sum, wantKg)
	}
	if sum["comparable"] != false {
		t.Fatalf("3 days of data must not be comparable with the previous month: %v", sum["comparable"])
	}
	if sci, _ := sum["sci_score"].(float64); math.Abs(sci-wantKg*1000/3000) > 0.05 {
		t.Fatalf("SCI = %v, want %.3f g per functional unit", sum["sci_score"], wantKg*1000/3000)
	}
	if len(items(c.must("GET", "/api/v1/carbon/trend?project_id="+proj, alice, "", 200))) != 3 {
		t.Error("trend must have 3 days")
	}
	if m := c.must("GET", "/api/v1/carbon/methodology", alice, "", 200); str(m, "version") != "CCF-2026.1" || str(m, "status") != "provisional" {
		t.Errorf("methodology: %v", m)
	}
	fin := c.must("GET", "/api/v1/finops/summary", alice, "", 200)
	if fin["previous_comparable"] != false {
		t.Fatalf("finops previous must not be comparable: %v", fin)
	}
	if fin["total_cost"].(float64) != 378 || str(fin, "currency") != "USD" {
		t.Fatalf("finops summary: %v", fin)
	}
	dp := items(c.must("GET", "/api/v1/dashboard/providers", alice, "", 200))
	if len(dp) != 1 || dp[0].(map[string]any)["provider"] != "aws" || dp[0].(map[string]any)["share_pct"].(float64) != 100 {
		t.Fatalf("providers: %v", dp)
	}
	svcs := items(c.must("GET", "/api/v1/dashboard/services", alice, "", 200))
	if len(svcs) != 4 {
		t.Fatalf("dashboard services = %d, want 4", len(svcs))
	}
	if tr := items(c.must("GET", "/api/v1/dashboard/trend", alice, "", 200)); len(tr) != 3 || tr[0].(map[string]any)["cost"].(float64) != 126 {
		t.Fatalf("dashboard trend: %v", tr)
	}
	if act := c.must("GET", "/api/v1/dashboard/activity", alice, "", 200); !strings.Contains(fmt.Sprint(act), "cloud_sync.completed") {
		t.Errorf("activity lacks the sync: %v", act)
	}

	// ---- recommendations: only into allowed regions, human approval, compliance re-check ----
	recs := items(c.must("GET", "/api/v1/recommendations?status=open", alice, "", 200))
	if len(recs) != 3 {
		t.Fatalf("recommendations = %d, want 3", len(recs))
	}
	top := recs[0].(map[string]any)
	if str(top, "type") != "region_shift" || str(top, "service_name") != "Amazon Elastic Compute Cloud - Compute" ||
		str(top, "recommended_region") != usw2 || str(top, "status") != "open" {
		t.Fatalf("top recommendation: %v", top)
	}
	if str(top, "cost_basis") != "not_estimated" || top["estimated_cost_impact"] != nil {
		t.Errorf("cost must not be invented: %v", top)
	}
	if cc := sub(top, "compliance_check"); str(cc, "residency") != "passed" {
		t.Errorf("compliance_check: %v", cc)
	}
	if conf, _ := top["confidence"].(float64); conf <= 0 || conf >= 0.2 {
		t.Errorf("3 days of data must give low confidence, got %v", top["confidence"])
	}
	for _, r := range recs {
		if str(r.(map[string]any), "recommended_region") == eun1 {
			t.Fatal("eu-north-1 is cleaner but not in the project's allow-list")
		}
	}
	recID := str(top, "id")
	c.must("POST", "/api/v1/recommendations/"+recID+"/apply", alice, "", 409) // open -> applied is not allowed
	c.must("POST", "/api/v1/recommendations/"+recID+"/approve", bob, "", 403) // bob has no membership yet
	if r := c.must("POST", "/api/v1/recommendations/"+recID+"/approve", alice, "", 200); str(r, "status") != "approved" {
		t.Fatalf("approve: %v", r)
	}
	if r := c.must("POST", "/api/v1/recommendations/"+recID+"/apply", alice, "", 200); str(r, "status") != "applied" {
		t.Fatalf("apply: %v", r)
	}
	c.must("POST", "/api/v1/recommendations/"+recID+"/approve", alice, "", 409) // applied is terminal
	c.must("POST", "/api/v1/recommendations/"+str(recs[2].(map[string]any), "id")+"/dismiss", alice, "", 200)
	// residency policy tightened after generation: the remaining finding can no longer be approved
	c.must("PUT", "/api/v1/projects/"+proj+"/policy", alice, fmt.Sprintf(`{"allowed_regions":["%s"]}`, use1), 200)
	c.must("POST", "/api/v1/recommendations/"+str(recs[1].(map[string]any), "id")+"/approve", alice, "", 409)

	// ---- members, invitations, role rules ----
	inv := c.must("POST", "/api/v1/invitations", alice, fmt.Sprintf(`{"email":"%s","role":"viewer"}`, bobEmail), 201)
	token := str(inv, "token")
	if !strings.HasPrefix(token, "inv_") {
		t.Fatalf("invitation token: %v", inv)
	}
	c.must("POST", "/api/v1/invitations", alice, `{"email":"x@example.com","role":"owner"}`, 422) // owners are never invited
	c.must("POST", "/api/v1/invitations/accept", eve, fmt.Sprintf(`{"token":"%s"}`, token), 422)  // wrong person
	c.must("POST", "/api/v1/invitations/accept", bob, fmt.Sprintf(`{"token":"%s"}`, token), 200)
	c.must("POST", "/api/v1/invitations/accept", bob, fmt.Sprintf(`{"token":"%s"}`, token), 422) // one-time
	c.must("GET", "/api/v1/projects", bob, "", 200)
	c.must("POST", "/api/v1/projects", bob, `{"name":"nope","functional_unit":"r"}`, 403) // viewer
	c.must("GET", "/api/v1/members", bob, "", 403)
	c.must("GET", "/api/v1/audit-log", bob, "", 403)
	c.must("GET", "/api/v1/finops/budgets", bob, "", 403)
	members := items(c.must("GET", "/api/v1/members", alice, "", 200))
	if len(members) != 2 {
		t.Fatalf("members = %d", len(members))
	}
	var bobMember, aliceMember string
	for _, m := range members {
		mm := m.(map[string]any)
		if str(mm, "email") == bobEmail {
			bobMember = str(mm, "id")
		} else {
			aliceMember = str(mm, "id")
		}
	}
	c.must("PATCH", "/api/v1/members/"+aliceMember, alice, `{"role":"admin"}`, 409) // last owner
	c.must("DELETE", "/api/v1/members/"+aliceMember, alice, "", 409)
	c.must("PATCH", "/api/v1/members/"+bobMember, alice, `{"role":"root"}`, 422)
	c.must("PATCH", "/api/v1/members/"+bobMember, alice, `{"role":"admin"}`, 200)
	c.must("PATCH", "/api/v1/members/"+aliceMember, bob, `{"role":"viewer"}`, 403) // an admin cannot demote an owner
	c.must("PATCH", "/api/v1/members/"+bobMember, alice, `{"role":"billing"}`, 200)
	c.must("POST", "/api/v1/finops/budgets", bob, `{"amount":500,"currency":"USD","period":"monthly"}`, 201) // billing manages budgets
	c.must("GET", "/api/v1/projects", bob, "", 403)                                                          // billing has no project access
	bud := items(c.must("GET", "/api/v1/finops/budgets", bob, "", 200))
	if len(bud) != 1 || bud[0].(map[string]any)["state"] == nil {
		t.Fatalf("budgets: %v", bud)
	}
	c.must("DELETE", "/api/v1/members/"+bobMember, alice, "", 204)
	if code, body, _ := c.do("GET", "/api/v1/projects", bob, ""); code != 403 || str(body, "detail") != "onboarding_required" {
		t.Fatalf("removed member: %d %v", code, body)
	}

	// ---- API keys and the CI gate ----
	k := c.must("POST", "/api/v1/api-keys", alice, `{"name":"github-actions","role":"ci"}`, 201)
	key := str(k, "key")
	if !strings.HasPrefix(key, "grk_") {
		t.Fatalf("api key: %v", k)
	}
	c.must("POST", "/api/v1/api-keys", alice, `{"name":"x","role":"admin"}`, 422)
	gate := c.must("POST", "/api/v1/ci/evaluate", key,
		fmt.Sprintf(`{"project_id":"%s","changes":[{"kind":"vcpu_hours","amount":100000,"region":"%s","monthly_cost_delta":40}],"thresholds":{"carbon_block_kg":10}}`, proj, use1), 200)
	if str(gate, "verdict") != "BLOCK" || gate["carbon_kg_month"].(float64) < 90 {
		t.Fatalf("gate: %v", gate)
	}
	small := c.must("POST", "/api/v1/ci/evaluate", key,
		fmt.Sprintf(`{"project_id":"%s","changes":[{"kind":"vcpu_hours","amount":10,"region":"%s","monthly_cost_delta":1}],"thresholds":{"carbon_warn_kg":1000,"carbon_block_kg":2000}}`, proj, use1), 200)
	if str(small, "verdict") != "PASS" {
		t.Fatalf("small change: %v", small)
	}
	c.must("GET", "/api/v1/projects", key, "", 403)                           // a CI key may only evaluate
	c.must("POST", "/api/v1/recommendations/"+recID+"/approve", key, "", 403) // machines never approve
	c.must("POST", "/api/v1/api-keys", key, `{"name":"x","role":"ci"}`, 403)
	keysRaw := c.must("GET", "/api/v1/api-keys", alice, "", 200)
	if strings.Contains(fmt.Sprint(keysRaw), "key_hash") || strings.Contains(fmt.Sprint(keysRaw), key) {
		t.Fatalf("API key material leaked in listing: %v", keysRaw)
	}
	keyID := str(items(keysRaw)[0].(map[string]any), "id")
	c.must("DELETE", "/api/v1/api-keys/"+keyID, alice, "", 204)
	c.must("POST", "/api/v1/ci/evaluate", key, fmt.Sprintf(`{"project_id":"%s","changes":[{"kind":"vcpu_hours","amount":1,"region":"%s"}]}`, proj, use1), 401)

	// ---- reports: queued, rendered by the worker, stored, downloaded ----
	rep := c.must("POST", "/api/v1/reports", alice, fmt.Sprintf(`{"kind":"carbon","format":"csv","period_start":"%s","period_end":"%s"}`,
		today.AddDate(0, 0, -5).Format(time.DateOnly), today.Format(time.DateOnly)), 202)
	pdfRep := c.must("POST", "/api/v1/reports", alice, fmt.Sprintf(`{"kind":"sci","format":"pdf","project_id":"%s","period_start":"%s","period_end":"%s"}`,
		proj, today.AddDate(0, 0, -5).Format(time.DateOnly), today.Format(time.DateOnly)), 202)
	c.must("GET", "/api/v1/reports/"+str(rep, "id")+"/download", alice, "", 409) // not rendered yet
	c.must("POST", "/api/v1/reports", alice, `{"kind":"nope","format":"csv","period_start":"2026-01-01","period_end":"2026-01-31"}`, 422)
	pump(t, mux, recA, recW)
	if r := c.must("GET", "/api/v1/reports/"+str(rep, "id"), alice, "", 200); str(r, "status") != "ready" {
		t.Fatalf("report: %v", r)
	}
	code, _, raw := c.do("GET", "/api/v1/reports/"+str(rep, "id")+"/download", alice, "")
	if code != 200 || !strings.Contains(string(raw), "carbon_kg_co2e") || !strings.Contains(string(raw), "Amazon Elastic Compute Cloud") || !strings.Contains(string(raw), "provisional") {
		t.Fatalf("csv download %d: %.200s", code, raw)
	}
	if code, _, raw := c.do("GET", "/api/v1/reports/"+str(pdfRep, "id")+"/download", alice, ""); code != 200 || !strings.HasPrefix(string(raw), "%PDF-") {
		t.Fatalf("pdf download %d", code)
	}

	// ---- audit trail ----
	log := fmt.Sprint(c.must("GET", "/api/v1/audit-log?limit=200", alice, "", 200))
	for _, action := range []string{"tenant.onboarded", "project.created", "project.policy_updated", "cloud_connection.created", "cloud_sync.completed",
		"recommendation.approved", "recommendation.applied", "recommendation.dismissed", "invitation.created", "member.joined",
		"member.role_changed", "member.removed", "api_key.created", "api_key.revoked", "report.generated", "budget.created"} {
		if !strings.Contains(log, action) {
			t.Errorf("audit log lacks %q", action)
		}
	}

	// ---- another tenant sees none of it ----
	carolTenant := str(c.must("POST", "/api/v1/onboarding", carol, `{"organization_name":"Other Inc"}`, 201), "tenant_id")
	if carolTenant == tenant {
		t.Fatal("two onboardings share a tenant")
	}
	for path, want := range map[string]int{
		"/api/v1/projects/" + proj:                        404,
		"/api/v1/cloud-accounts/" + connID:                404,
		"/api/v1/recommendations/" + recID:                404,
		"/api/v1/reports/" + str(rep, "id") + "/download": 404,
		"/api/v1/reports/" + str(rep, "id"):               404,
	} {
		if code, _, _ := c.do("GET", path, carol, ""); code != want {
			t.Errorf("carol GET %s = %d, want %d", path, code, want)
		}
	}
	for _, path := range []string{"/api/v1/projects", "/api/v1/usage", "/api/v1/cloud-accounts", "/api/v1/recommendations", "/api/v1/reports", "/api/v1/dashboard/trend", "/api/v1/dashboard/providers"} {
		if n := len(items(c.must("GET", path, carol, "", 200))); n != 0 {
			t.Errorf("carol sees %d items at %s", n, path)
		}
	}
	if s := c.must("GET", "/api/v1/carbon/summary", carol, "", 200); s["has_data"] != false || s["sci_score"] != nil {
		t.Errorf("empty tenant summary: %v", s)
	}
	if code, _, _ := c.do("POST", "/api/v1/projects/"+proj+"/functional-units", carol, `{"period_start":"2026-01-01","period_end":"2026-01-31","units":1}`); code != 404 {
		t.Errorf("carol wrote functional units into alice's project: %d", code)
	}
}
