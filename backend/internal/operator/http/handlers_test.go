package http

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/hibiken/asynq"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/queue"
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
func (f *fakeRepo) CarbonRegionSources(context.Context) ([]domain.RegionSourceRow, error) {
	return []domain.RegionSourceRow{{Region: "eu-central-1", Provider: "ember", Months: 24}}, nil
}
func (f *fakeRepo) IntensityProvenance(context.Context, int) ([]domain.ProvenanceRow, error) {
	return []domain.ProvenanceRow{{Source: "electricitymaps", Basis: "day_mean", Rows: 3}, {Rows: 1}}, nil
}
func (f *fakeRepo) SourceDivergence(context.Context, int) ([]domain.DivergenceRow, error) {
	now := time.Now()
	return []domain.DivergenceRow{
		{Region: "sa-east-1", Provider: "electricitymaps", G: 300, N: 40, Newest: now},
		{Region: "sa-east-1", Provider: "ons", G: 90, N: 150, Newest: now},
		{Region: "sa-east-1", Provider: "ember", G: 105, N: 1, Newest: now.AddDate(0, -2, 0), Kind: "monthly"},
	}, nil
}
func (f *fakeRepo) OrgUsage(context.Context, int) ([]domain.OrgUsage, error) {
	return []domain.OrgUsage{{TenantID: t1, Name: "Acme", UsageRows: 2_000_000, SyncRuns: 100, Reports: 10}, {TenantID: d1, Name: "Small", UsageRows: 1000}}, nil
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
	for _, p := range []string{"/operator/tenants", "/operator/notifications/stats", "/operator/notifications/deliveries?status=failed", "/operator/connections/unhealthy", "/operator/system", "/operator/audit?limit=20", "/operator/data-sources", "/operator/operators", "/operator/plans", "/operator/billing/events", "/operator/methodology", "/operator/api-usage", "/operator/carbon/regions", "/operator/usage", "/operator/aggregation"} {
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
	if len(repo.log) != 16 { // every successful read is recorded; refused writes are not
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

func TestRegionSourcesPickTheFirstTrustedSourceThatHasData(t *testing.T) {
	repo := &fakeRepo{}
	svc := application.Service{Repo: repo}
	got, err := svc.RegionSources(context.Background())
	if err != nil || len(got) != 2 { // eu-central-1 from the sources, us-east-1 only as a region in use
		t.Fatalf("%+v %v", got, err)
	}
	for _, r := range got {
		switch r.Region {
		case "eu-central-1":
			if r.Effective != "ember" {
				t.Errorf("only Ember has data for eu-central-1: %+v", r)
			}
		case "us-east-1":
			if r.Effective != "" || len(r.Sources) != 0 {
				t.Errorf("a region with no data must say so: %+v", r)
			}
		}
	}
}

func TestServingCostIsCountsTimesUnitCostsAndSortedByCost(t *testing.T) {
	c := application.UnitCosts{Base: 5, UsageRowsPerMillion: 2, SyncRun: 0.01, Report: 0.02}
	u := domain.OrgUsage{UsageRows: 2_000_000, SyncRuns: 100, Reports: 10}
	if got, want := c.Cost(u), 5+4+1+0.2; got < want-1e-9 || got > want+1e-9 {
		t.Errorf("cost = %v, want %v", got, want)
	}
	svc := application.Service{Repo: &fakeRepo{}}
	items, _, err := svc.OrgUsage(context.Background(), 30)
	if err != nil || len(items) != 2 || items[0].Name != "Acme" {
		t.Errorf("%+v %v", items, err)
	}
	if _, _, err := svc.OrgUsage(context.Background(), 0); err == nil {
		t.Error("days must be validated")
	}
}

func TestRecalculateAllIsAdminOnlyAndBounded(t *testing.T) {
	post := func(h http.Handler, body string) int {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequestWithContext(context.Background(), "POST", "/operator/carbon/recalculate", strings.NewReader(body)))
		return rec.Code
	}
	_, h := setup("viewer-1")
	if c := post(h, `{"days":30}`); c != 403 {
		t.Errorf("viewer = %d, want 403", c)
	}
	_, h = setup("admin-1")
	for _, b := range []string{`{"days":0}`, `{"days":999}`, `{}`} {
		if c := post(h, b); c != 422 {
			t.Errorf("%s = %d, want 422", b, c)
		}
	}
}

func TestAggregationAndExplain(t *testing.T) {
	_, h := setup("viewer-1")
	rec := do(h, "GET", "/operator/aggregation")
	var a domain.Aggregation
	if rec.Code != 200 || json.Unmarshal(rec.Body.Bytes(), &a) != nil {
		t.Fatalf("aggregation = %d %s", rec.Code, rec.Body)
	}
	if len(a.Sources) != 9 || len(a.Rules) == 0 || a.Policy.MinDayReadings != 6 || len(a.Policy.Priority) != 9 {
		t.Errorf("unexpected shape: %+v", a)
	}
	// 3 of the 4 rows have an origin and all of those are a mean of the day itself; the empty source is not guessed
	if a.Provenance.Total != 4 || a.Provenance.Unattributed != 1 || a.Provenance.PreciseShare != 1 {
		t.Errorf("provenance: %+v", a.Provenance)
	}
	// without an engine behind it the explanation is not available; bad input is refused before that matters
	if rec := do(h, "GET", "/operator/aggregation/explain?region=eu-west-2&day=2026-09-01"); rec.Code != 404 {
		t.Errorf("explain without an engine = %d", rec.Code)
	}
}

func TestExplainValidatesItsQuery(t *testing.T) {
	repo := &fakeRepo{roles: map[string]domain.Role{"viewer-1": domain.Viewer}}
	called := 0
	svc := application.Service{Repo: repo, System: func() domain.System { return domain.System{} },
		Explain: func(_ context.Context, region string, day time.Time) (any, error) {
			called++
			return map[string]string{"region": region, "day": day.Format(time.DateOnly)}, nil
		}}
	r := chi.NewRouter()
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			next.ServeHTTP(w, req.WithContext(auth.WithClaims(req.Context(), auth.Claims{Subject: "viewer-1"})))
		})
	})
	Handlers{Svc: svc}.IdentityRoutes(r)
	for _, bad := range []string{"", "?region=eu-west-2", "?day=2026-09-01", "?region=x&day=2026-09-01", "?region=eu-west-2&day=nope", "?region=a%20b%3Bdrop&day=2026-09-01", "?region=eu-west-2&day=1999-01-01"} {
		if rec := do(r, "GET", "/operator/aggregation/explain"+bad); rec.Code != 400 {
			t.Errorf("%q = %d, want 400", bad, rec.Code)
		}
	}
	if called != 0 {
		t.Fatal("the engine was asked with an invalid query")
	}
	day := time.Now().AddDate(0, 0, -3).Format(time.DateOnly)
	if rec := do(r, "GET", "/operator/aggregation/explain?region=eu-west-2&day="+day); rec.Code != 200 || called != 1 || !strings.Contains(rec.Body.String(), day) {
		t.Errorf("valid query = %d %s (%d calls)", rec.Code, rec.Body, called)
	}
}

type fakeJobs struct {
	retried, deleted []string
	err              error
}

func (f *fakeJobs) Queues() ([]queue.QueueStat, error)     { return nil, nil }
func (f *fakeJobs) Failed(int) ([]queue.FailedTask, error) { return nil, nil }
func (f *fakeJobs) Schedules() ([]queue.Schedule, error)   { return nil, nil }
func (f *fakeJobs) Retry(q, id string) error               { f.retried = append(f.retried, q+"/"+id); return f.err }
func (f *fakeJobs) Delete(q, id string) error              { f.deleted = append(f.deleted, q+"/"+id); return f.err }

func TestFailedJobActions(t *testing.T) {
	jobs := &fakeJobs{}
	mk := func(sub string) http.Handler {
		repo := &fakeRepo{roles: map[string]domain.Role{"viewer-1": domain.Viewer, "admin-1": domain.Admin}}
		h := Handlers{Svc: application.Service{Repo: repo, System: func() domain.System { return domain.System{} }, Jobs: jobs}}
		r := chi.NewRouter()
		r.Use(func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				next.ServeHTTP(w, req.WithContext(auth.WithClaims(req.Context(), auth.Claims{Subject: sub})))
			})
		})
		h.IdentityRoutes(r)
		return r
	}
	const id = "3b29c3255b2adac79b6aa8af"
	if rec := do(mk("viewer-1"), "POST", "/operator/jobs/default/"+id+"/retry"); rec.Code != 403 {
		t.Errorf("a viewer changed a task: %d", rec.Code)
	}
	admin := mk("admin-1")
	for _, bad := range []string{"/operator/jobs/DEF%20ault/" + id + "/retry", "/operator/jobs/default/x/retry", "/operator/jobs/default/..%2Fup/delete"} {
		if rec := do(admin, "POST", bad); rec.Code != 404 {
			t.Errorf("%s = %d, want 404", bad, rec.Code)
		}
	}
	if len(jobs.retried)+len(jobs.deleted) != 0 {
		t.Fatal("the queue was touched by an invalid request")
	}
	if rec := do(admin, "POST", "/operator/jobs/default/"+id+"/retry"); rec.Code != 200 || len(jobs.retried) != 1 || jobs.retried[0] != "default/"+id {
		t.Errorf("retry = %d %v", rec.Code, jobs.retried)
	}
	if rec := do(admin, "POST", "/operator/jobs/default/"+id+"/delete"); rec.Code != 200 || len(jobs.deleted) != 1 {
		t.Errorf("delete = %d %v", rec.Code, jobs.deleted)
	}
	jobs.err = asynq.ErrTaskNotFound
	if rec := do(admin, "POST", "/operator/jobs/default/"+id+"/delete"); rec.Code != 404 {
		t.Errorf("a task that is gone = %d, want 404", rec.Code)
	}
	jobs.err = errors.New("task is active")
	if rec := do(admin, "POST", "/operator/jobs/default/"+id+"/delete"); rec.Code != 409 {
		t.Errorf("a task that is running = %d, want 409", rec.Code)
	}
}
