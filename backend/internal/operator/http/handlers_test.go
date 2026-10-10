package http

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/quixgit/greenops-reliabilix/backend/internal/operator/application"
	"github.com/quixgit/greenops-reliabilix/backend/internal/operator/domain"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/auth"
)

func (f *fakeRepo) Errors(_ context.Context, status, service string, _ int) ([]domain.ErrorGroup, error) {
	f.errFilters = append(f.errFilters, status+"/"+service)
	return []domain.ErrorGroup{{ID: "e1", Service: "worker", Component: "job", Severity: "error", Message: "boom", Status: "open", Occurrences: 3}}, nil
}
func (f *fakeRepo) SetErrorStatus(_ context.Context, actor, id, status string) (bool, error) {
	f.errSet = append(f.errSet, actor+"/"+id+"/"+status)
	return f.retryOK, nil
}

type fakeRepo struct {
	plans      []string
	errFilters []string
	errSet     []string
	roles      map[string]domain.Role
	log        []string
	logErr     error
	retried    []string
	retryOK    bool
	toggled    []string
	statuses   []string
}

func (f *fakeRepo) Role(_ context.Context, sub string) (domain.Role, bool, error) {
	r, ok := f.roles[sub]
	return r, ok, nil
}
func (f *fakeRepo) LogAccess(_ context.Context, sub, action string) error {
	if f.logErr != nil {
		return f.logErr
	}
	f.log = append(f.log, sub+" "+action)
	return nil
}
func (f *fakeRepo) Tenants(context.Context, int) ([]domain.Tenant, error) {
	return []domain.Tenant{{ID: "t1", Name: "Acme", CreatedAt: time.Unix(0, 0).UTC()}}, nil
}
func (f *fakeRepo) Deliveries(_ context.Context, status string, _ int) ([]domain.Delivery, error) {
	f.statuses = append(f.statuses, status)
	return []domain.Delivery{}, nil
}
func (f *fakeRepo) DeliveryStats(context.Context, int) ([]domain.DeliveryStat, error) {
	return []domain.DeliveryStat{{Kind: "webhook", Status: "sent", Count: 3}}, nil
}
func (f *fakeRepo) UnhealthyConnections(context.Context, int) ([]domain.Connection, error) {
	return []domain.Connection{}, nil
}
func (f *fakeRepo) Audit(context.Context, int) ([]domain.AuditEntry, error) {
	return []domain.AuditEntry{{ID: 1, OperatorSub: "viewer-1", Action: "GET /operator/me"}}, nil
}
func (f *fakeRepo) ProviderUsage(context.Context) ([]domain.ProviderUsage, error) {
	return []domain.ProviderUsage{{Provider: "aws", Connections: 3, Organizations: 2, InError: 1}}, nil
}
func (f *fakeRepo) Operators(context.Context) ([]domain.OperatorRecord, error) {
	return []domain.OperatorRecord{{Subject: "admin-1", Role: domain.Admin}}, nil
}
func (f *fakeRepo) SetSubscription(_ context.Context, actor, tenant, plan, status string, _, _ *time.Time, reason string) (bool, error) {
	f.plans = append(f.plans, actor+"/"+tenant+"/"+plan+"/"+status+"/"+reason)
	return true, nil
}
func (f *fakeRepo) Subscription(context.Context, string) (domain.Subscription, error) {
	return domain.Subscription{Status: "none"}, nil
}
func (f *fakeRepo) BillingEvents(context.Context, int) ([]domain.BillingEvent, error) {
	return []domain.BillingEvent{}, nil
}
func (f *fakeRepo) AssignOperator(_ context.Context, actor, email string, role domain.Role) (string, error) {
	f.plans = append(f.plans, "assign:"+actor+"/"+email+"/"+string(role))
	return "sub", nil
}
func (f *fakeRepo) WithdrawOperator(_ context.Context, actor, sub string) (bool, error) {
	f.plans = append(f.plans, "withdraw:"+actor+"/"+sub)
	return true, nil
}
func (f *fakeRepo) CarbonRegions(context.Context) ([]string, error) {
	return []string{"eu-central-1", "us-east-1"}, nil
}
func (f *fakeRepo) APIUsage(context.Context, int) ([]domain.APIUsage, error) {
	return []domain.APIUsage{{Provider: "electricitymaps", Requests: 12}}, nil
}
func (f *fakeRepo) RetryDelivery(_ context.Context, actor, tenant, id string) (bool, error) {
	f.retried = append(f.retried, actor+"/"+tenant+"/"+id)
	return f.retryOK, nil
}
func (f *fakeRepo) SetDestinationEnabled(_ context.Context, actor, tenant, id string, en bool) (bool, error) {
	f.toggled = append(f.toggled, actor+"/"+tenant+"/"+id)
	return true, nil
}

const (
	t1 = "11111111-1111-1111-1111-111111111111"
	d1 = "22222222-2222-2222-2222-222222222222"
)

func setup(sub string) (*fakeRepo, http.Handler) {
	repo := &fakeRepo{roles: map[string]domain.Role{"viewer-1": domain.Viewer, "admin-1": domain.Admin}, retryOK: true}
	h := Handlers{Svc: application.Service{Repo: repo, System: func() domain.System { return domain.System{Version: "v", Env: "dev"} }}}
	r := chi.NewRouter()
	r.Use(func(next http.Handler) http.Handler { // stands in for AuthenticateIdentity
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			if sub != "" {
				req = req.WithContext(auth.WithClaims(req.Context(), auth.Claims{Subject: sub, Email: sub + "@example.com"}))
			}
			next.ServeHTTP(w, req)
		})
	})
	h.IdentityRoutes(r)
	return repo, r
}

func do(h http.Handler, method, path string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequestWithContext(context.Background(), method, path, strings.NewReader("")))
	return rec
}

func TestNonOperatorsAreRefusedEverywhere(t *testing.T) {
	for _, sub := range []string{"", "someone-else", "apikey:abc"} {
		repo, h := setup(sub)
		for _, route := range []struct{ m, p string }{
			{"GET", "/operator/me"}, {"GET", "/operator/tenants"}, {"GET", "/operator/system"}, {"GET", "/operator/notifications/deliveries"}, {"GET", "/operator/audit"}, {"GET", "/operator/data-sources"}, {"GET", "/operator/jobs"}, {"GET", "/operator/operators"},
			{"POST", "/operator/tenants/" + t1 + "/deliveries/" + d1 + "/retry"},
		} {
			want := http.StatusForbidden
			if sub == "" {
				want = http.StatusUnauthorized
			}
			if rec := do(h, route.m, route.p); rec.Code != want {
				t.Errorf("sub=%q %s %s = %d, want %d", sub, route.m, route.p, rec.Code, want)
			}
		}
		if len(repo.log) != 0 || len(repo.retried) != 0 {
			t.Errorf("sub=%q: a refused caller must leave no trace and change nothing: %v %v", sub, repo.log, repo.retried)
		}
	}
}

func TestViewerReadsButCannotAct(t *testing.T) {
	repo, h := setup("viewer-1")
	rec := do(h, "GET", "/operator/me")
	var me domain.Operator
	if rec.Code != 200 || json.Unmarshal(rec.Body.Bytes(), &me) != nil || me.Role != domain.Viewer || me.Subject != "viewer-1" {
		t.Fatalf("me = %d %s", rec.Code, rec.Body)
	}
	for _, p := range []string{"/operator/tenants", "/operator/notifications/stats", "/operator/notifications/deliveries?status=failed", "/operator/connections/unhealthy", "/operator/system", "/operator/audit?limit=20", "/operator/data-sources", "/operator/operators", "/operator/plans", "/operator/billing/events", "/operator/methodology", "/operator/api-usage"} {
		if rec := do(h, "GET", p); rec.Code != 200 {
			t.Errorf("GET %s = %d", p, rec.Code)
		}
	}
	for _, p := range []string{"/retry", "/enable", "/disable"} {
		path := "/operator/tenants/" + t1 + "/deliveries/" + d1 + "/retry"
		if p != "/retry" {
			path = "/operator/tenants/" + t1 + "/destinations/" + d1 + p
		}
		if rec := do(h, "POST", path); rec.Code != 403 {
			t.Errorf("viewer POST %s = %d, want 403", path, rec.Code)
		}
	}
	if len(repo.retried)+len(repo.toggled) != 0 {
		t.Error("a read-only operator changed something")
	}
	if len(repo.log) != 13 { // every successful read is recorded; refused writes are not
		t.Errorf("access log = %v", repo.log)
	}
}

func TestAdminActions(t *testing.T) {
	repo, h := setup("admin-1")
	if rec := do(h, "POST", "/operator/tenants/"+t1+"/deliveries/"+d1+"/retry"); rec.Code != 202 {
		t.Errorf("retry = %d %s", rec.Code, rec.Body)
	}
	if len(repo.retried) != 1 || repo.retried[0] != "admin-1/"+t1+"/"+d1 {
		t.Errorf("retried = %v", repo.retried)
	}
	repo.retryOK = false
	if rec := do(h, "POST", "/operator/tenants/"+t1+"/deliveries/"+d1+"/retry"); rec.Code != 404 {
		t.Errorf("retry of a delivery that cannot be retried = %d, want 404", rec.Code)
	}
	if rec := do(h, "POST", "/operator/tenants/"+t1+"/destinations/"+d1+"/disable"); rec.Code != 200 || !strings.Contains(rec.Body.String(), `"enabled":false`) {
		t.Errorf("disable = %d %s", rec.Code, rec.Body)
	}
	if rec := do(h, "POST", "/operator/tenants/"+t1+"/destinations/"+d1+"/enable"); rec.Code != 200 || !strings.Contains(rec.Body.String(), `"enabled":true`) {
		t.Errorf("enable = %d %s", rec.Code, rec.Body)
	}
	if rec := do(h, "POST", "/operator/tenants/not-a-uuid/deliveries/"+d1+"/retry"); rec.Code != 404 {
		t.Errorf("bad tenant id = %d, want 404", rec.Code)
	}
	if len(repo.log) < 5 || !strings.HasPrefix(repo.log[0], "admin-1 POST /operator/tenants/") {
		t.Errorf("actions must be logged with method and path: %v", repo.log)
	}
}

func TestNoAccessWithoutARecord(t *testing.T) {
	repo, h := setup("viewer-1")
	repo.logErr = errors.New("log table unavailable")
	if rec := do(h, "GET", "/operator/tenants"); rec.Code != 500 || strings.Contains(rec.Body.String(), "Acme") {
		t.Errorf("when the access cannot be recorded nothing may be shown: %d %s", rec.Code, rec.Body)
	}
}

func TestQueryValidation(t *testing.T) {
	_, h := setup("viewer-1")
	for p, want := range map[string]int{
		"/operator/notifications/deliveries?status=bogus":  400,
		"/operator/notifications/deliveries?limit=0":       400,
		"/operator/notifications/deliveries?limit=201":     400,
		"/operator/notifications/stats?hours=0":            400,
		"/operator/notifications/stats?hours=abc":          400,
		"/operator/notifications/stats?hours=721":          400,
		"/operator/notifications/stats?hours=720":          200,
		"/operator/notifications/deliveries?status=failed": 200,
	} {
		if rec := do(h, "GET", p); rec.Code != want {
			t.Errorf("GET %s = %d, want %d", p, rec.Code, want)
		}
	}
}

func TestErrorLogEndpoints(t *testing.T) {
	repo, h := setup("viewer-1")
	// default: open errors only; "all" lifts the filter; filters reach the repository untouched
	do(h, "GET", "/operator/errors")
	do(h, "GET", "/operator/errors?status=all&service=worker")
	do(h, "GET", "/operator/errors?status=resolved")
	if got := strings.Join(repo.errFilters, ","); got != "open/,/worker,resolved/" {
		t.Errorf("filters = %s", got)
	}
	for p, want := range map[string]int{"/operator/errors?status=bogus": 400, "/operator/errors?service=Bad!": 400, "/operator/errors?limit=0": 400, "/operator/errors?limit=500": 400} {
		if rec := do(h, "GET", p); rec.Code != want {
			t.Errorf("GET %s = %d, want %d", p, rec.Code, want)
		}
	}
	e1 := "33333333-3333-3333-3333-333333333333"
	if rec := do(h, "POST", "/operator/errors/"+e1+"/resolve"); rec.Code != 403 || len(repo.errSet) != 0 {
		t.Errorf("a viewer must not change error status: %d %v", rec.Code, repo.errSet)
	}
	repo2, h2 := setup("admin-1")
	repo2.retryOK = true
	for _, a := range []string{"resolve", "ignore", "reopen"} {
		if rec := do(h2, "POST", "/operator/errors/"+e1+"/"+a); rec.Code != 200 {
			t.Errorf("%s = %d", a, rec.Code)
		}
	}
	if got := strings.Join(repo2.errSet, ","); got != "admin-1/"+e1+"/resolved,admin-1/"+e1+"/ignored,admin-1/"+e1+"/open" {
		t.Errorf("status changes = %s", got)
	}
	repo2.retryOK = false
	if rec := do(h2, "POST", "/operator/errors/"+e1+"/resolve"); rec.Code != 404 {
		t.Errorf("unknown group = %d, want 404", rec.Code)
	}
	if rec := do(h2, "POST", "/operator/errors/nope/resolve"); rec.Code != 404 {
		t.Errorf("bad id = %d, want 404", rec.Code)
	}
}

func TestDataSourcesAreMergedWithRealConnectionCounts(t *testing.T) {
	repo, _ := setup("viewer-1")
	svc := application.Service{Repo: repo, Sources: func(context.Context) []domain.DataSource {
		return []domain.DataSource{
			{ID: "aws-cost", Name: "AWS Cost Explorer", State: "not_configured"},
			{ID: "gcp-billing", Name: "GCP billing export", State: "not_configured"},
			{ID: "wattime", Name: "WattTime", State: "planned"},
		}
	}}
	got, err := svc.DataSources(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if aws := got[0]; aws.Connections == nil || *aws.Connections != 3 || aws.State != "attention" {
		t.Errorf("aws source = %+v, want 3 connections and state attention (one connection fails)", aws)
	}
	if gcp := got[1]; gcp.Connections == nil || *gcp.Connections != 0 || gcp.State != "not_configured" {
		t.Errorf("gcp source = %+v, want zero connections, nothing invented", gcp)
	}
	if w := got[2]; w.Connections != nil || w.State != "planned" {
		t.Errorf("planned source must not carry counts: %+v", w)
	}
}

func TestAuditLimitIsValidated(t *testing.T) {
	_, h := setup("viewer-1")
	for _, q := range []string{"0", "501", "x"} {
		if rec := do(h, "GET", "/operator/audit?limit="+q); rec.Code != 400 {
			t.Errorf("limit=%s = %d, want 400", q, rec.Code)
		}
	}
}

func TestJobsIs404WithoutAQueueAndServesTelemetryWhenWired(t *testing.T) {
	_, h := setup("viewer-1")
	if rec := do(h, "GET", "/operator/jobs"); rec.Code != 404 {
		t.Errorf("no queue wired = %d, want 404", rec.Code)
	}
}

func TestAssigningAPlanNeedsAdminAKnownPlanAndAReason(t *testing.T) {
	put := func(h http.Handler, body string) int {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequestWithContext(context.Background(), "PUT", "/operator/tenants/"+t1+"/plan", strings.NewReader(body)))
		return rec.Code
	}
	repo, h := setup("viewer-1")
	if c := put(h, `{"plan":"team","reason":"upgrade"}`); c != 403 {
		t.Errorf("viewer = %d, want 403", c)
	}
	repo, h = setup("admin-1")
	for _, body := range []string{`{"plan":"nope","reason":"upgrade"}`, `{"plan":"team","reason":""}`, `{"plan":"team"}`} {
		if c := put(h, body); c != 422 {
			t.Errorf("%s = %d, want 422", body, c)
		}
	}
	if len(repo.plans) != 0 {
		t.Errorf("a refused request changed something: %v", repo.plans)
	}
}

func TestOperatorRolesAreAdminOnlyAndValidated(t *testing.T) {
	send := func(h http.Handler, method, path, body string) int {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequestWithContext(context.Background(), method, path, strings.NewReader(body)))
		return rec.Code
	}
	repo, h := setup("viewer-1")
	if c := send(h, "PUT", "/operator/operators", `{"email":"a@example.com","role":"admin"}`); c != 403 {
		t.Errorf("viewer assigning = %d, want 403", c)
	}
	if c := send(h, "POST", "/operator/operators/withdraw", `{"subject":"x"}`); c != 403 {
		t.Errorf("viewer withdrawing = %d, want 403", c)
	}
	repo, h = setup("admin-1")
	for _, body := range []string{`{"email":"not-an-address","role":"admin"}`, `{"email":"a@example.com","role":"root"}`, `{"role":"viewer"}`} {
		if c := send(h, "PUT", "/operator/operators", body); c != 422 {
			t.Errorf("%s = %d, want 422", body, c)
		}
	}
	if len(repo.plans) != 0 {
		t.Errorf("a refused request changed something: %v", repo.plans)
	}
	if c := send(h, "PUT", "/operator/operators", `{"email":"new@example.com","role":"viewer"}`); c != 200 {
		t.Errorf("valid assignment = %d, want 200", c)
	}
	if len(repo.plans) != 1 || repo.plans[0] != "assign:admin-1/new@example.com/viewer" {
		t.Errorf("assignment = %v", repo.plans)
	}
}

func TestBackfillPlanIsBoundedAndDryRunStartsNothing(t *testing.T) {
	post := func(h http.Handler, body string) (int, string) {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequestWithContext(context.Background(), "POST", "/operator/carbon/backfill", strings.NewReader(body)))
		return rec.Code, rec.Body.String()
	}
	_, h := setup("viewer-1")
	if c, _ := post(h, `{"from":"2026-01-01","to":"2026-02-01","dry_run":true}`); c != 403 {
		t.Errorf("viewer = %d, want 403", c)
	}
	_, h = setup("admin-1")
	c, body := post(h, `{"from":"2026-01-01","to":"2026-02-01","dry_run":true}`)
	if c != 200 || !strings.Contains(body, `"estimated_requests":8`) || !strings.Contains(body, `"queued":false`) { // 2 regions x ceil(31/9)=4
		t.Errorf("dry run = %d %s", c, body)
	}
	for _, b := range []string{`{"from":"2026-02-01","to":"2026-01-01"}`, `{"from":"2024-01-01","to":"2026-01-01"}`, `{"from":"x","to":"y"}`} {
		if c, _ := post(h, b); c != 422 {
			t.Errorf("%s = %d, want 422", b, c)
		}
	}
}
