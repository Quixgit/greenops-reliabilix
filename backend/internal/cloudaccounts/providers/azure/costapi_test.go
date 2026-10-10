package azure

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/quixgit/greenops-reliabilix/backend/internal/cloudaccounts/domain"
)

const (
	spTenant = "99999999-8888-7777-6666-555555555555"
	spClient = "b8b97d8b-fec7-4b74-a232-b0b605ff0959"
	spSub    = "11111111-2222-3333-4444-555555555555"
)

func spConn() domain.Connection {
	return domain.Connection{Provider: domain.Azure, AccountRef: spSub, ExternalID: "rlx-abc", SealedSecret: "sealed",
		CredentialRef: "azcost://" + spTenant + "@" + spClient}
}

// fakeAzure is a token endpoint and a Cost Management endpoint in one server.
type fakeAzure struct {
	srv       *httptest.Server
	queries   []map[string]any
	throttle  int32 // answer 429 this many times first
	forbidden bool
	tokens    int32
}

func newFakeAzure(t *testing.T) *fakeAzure {
	f := &fakeAzure{}
	mux := http.NewServeMux()
	mux.HandleFunc("/"+spTenant+"/oauth2/v2.0/token", func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&f.tokens, 1)
		_ = r.ParseForm()
		if r.Form.Get("client_id") != spClient || r.Form.Get("client_secret") != "the-secret" || r.Form.Get("scope") != "https://management.azure.com/.default" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		_, _ = w.Write([]byte(`{"access_token":"arm-token","expires_in":3600}`))
	})
	mux.HandleFunc("/subscriptions/"+spSub+"/providers/Microsoft.CostManagement/query", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer arm-token" || r.URL.Query().Get("api-version") == "" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		if f.forbidden {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		if atomic.AddInt32(&f.throttle, -1) >= 0 {
			w.Header().Set("Retry-After", "1")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		f.queries = append(f.queries, body)
		grouped := body["dataset"].(map[string]any)["grouping"] != nil
		w.Header().Set("Content-Type", "application/json")
		amortized := body["type"] == "AmortizedCost"
		if r.URL.Query().Get("page") == "2" {
			fmt.Fprint(w, `{"properties":{"nextLink":null,"columns":[{"name":"Cost"},{"name":"UsageDate"},{"name":"ServiceName"},{"name":"ResourceLocation"},{"name":"Currency"}],"rows":[[1.5,20261002,"Storage","EU West","EUR"]]}}`)
			return
		}
		switch {
		case !grouped:
			fmt.Fprint(w, `{"properties":{"columns":[{"name":"Cost"},{"name":"UsageDate"},{"name":"Currency"}],"rows":[[13.5,20261001,"EUR"],[1.5,20261002,"EUR"]]}}`)
		case amortized:
			fmt.Fprint(w, `{"properties":{"columns":[{"name":"Cost"},{"name":"UsageDate"},{"name":"ServiceName"},{"name":"ResourceLocation"},{"name":"Currency"}],"rows":[[9,20261001,"Virtual Machines","EU West","EUR"]]}}`)
		default:
			fmt.Fprintf(w, `{"properties":{"nextLink":%q,"columns":[{"name":"Cost"},{"name":"UsageDate"},{"name":"ServiceName"},{"name":"ResourceLocation"},{"name":"Currency"}],"rows":[[12,20261001,"Virtual Machines","EU West","EUR"],[1.5,20261001,"Bandwidth","EU West","EUR"]]}}`,
				f.srv.URL+"/subscriptions/"+spSub+"/providers/Microsoft.CostManagement/query?api-version=2023-11-01&page=2")
		}
	})
	f.srv = httptest.NewServer(mux)
	t.Cleanup(f.srv.Close)
	return f
}

func (f *fakeAzure) provider() *Provider {
	h := NoRedirects(http.DefaultClient)
	return &Provider{
		HTTP: h, Authority: f.srv.URL,
		Cost:       &CostAPI{HTTP: h, ARM: f.srv.URL, Sleep: func(context.Context, time.Duration) error { return nil }},
		OpenSecret: func(c domain.Connection) (string, error) { return "the-secret", nil },
	}
}

func TestServicePrincipalUsageAndControlTotals(t *testing.T) {
	f := newFakeAzure(t)
	p := f.provider()
	req := domain.UsageRequest{Connection: spConn(), From: time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC), To: time.Date(2026, 10, 3, 0, 0, 0, 0, time.UTC)}

	batches, err := p.GetUsage(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if len(batches) != 1 || batches[0].Provider != "azure" || batches[0].Source != SourceCostManagement {
		t.Fatalf("batches: %+v", batches)
	}
	var got struct {
		Rows []struct {
			Day, Service, Location, Currency string
			Billed, Effective                float64
		}
	}
	if err := json.Unmarshal(batches[0].Payload, &got); err != nil {
		t.Fatal(err)
	}
	by := map[string][2]float64{}
	for _, r := range got.Rows {
		by[r.Day+"|"+r.Service] = [2]float64{r.Billed, r.Effective}
	}
	if by["2026-10-01|Virtual Machines"] != [2]float64{12, 9} {
		t.Errorf("vm line: %v (all %v)", by["2026-10-01|Virtual Machines"], by)
	}
	if by["2026-10-01|Bandwidth"][0] != 1.5 || by["2026-10-02|Storage"][0] != 1.5 {
		t.Errorf("the second page and the other services must be included: %v", by)
	}
	// the window is sent as inclusive start and end-of-last-day timestamps
	tp := f.queries[0]["timePeriod"].(map[string]any)
	if tp["from"] != "2026-10-01T00:00:00Z" || tp["to"] != "2026-10-02T23:59:59Z" {
		t.Errorf("time period = %v", tp)
	}

	totals, err := p.GetControlTotals(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if len(totals) != 2 || totals[0].Day != "2026-10-01" || totals[0].Billed != 13.5 || totals[0].Currency != "EUR" {
		t.Errorf("control totals: %+v", totals)
	}
}

func TestServicePrincipalValidateAndThrottling(t *testing.T) {
	f := newFakeAzure(t)
	p := f.provider()
	atomic.StoreInt32(&f.throttle, 2) // two 429s with Retry-After, then success
	if err := p.Validate(context.Background(), spConn()); err != nil {
		t.Fatalf("throttling must be waited out: %v", err)
	}
	f.forbidden = true
	if err := p.Validate(context.Background(), spConn()); !errors.Is(err, domain.ErrAccessDenied) {
		t.Errorf("a missing role must read as access denied: %v", err)
	}
	f.forbidden = false
	atomic.StoreInt32(&f.throttle, 99)
	if err := p.Validate(context.Background(), spConn()); err == nil || errors.Is(err, domain.ErrAccessDenied) {
		t.Errorf("endless throttling is a retryable failure, not a customer error: %v", err)
	}
}

func TestRefusedAmortizedViewFallsBackToBilled(t *testing.T) {
	arm := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body["type"] == "AmortizedCost" {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprint(w, `{"error":{"code":"BadRequest","message":"not supported for this offer"}}`)
			return
		}
		fmt.Fprint(w, `{"properties":{"columns":[{"name":"Cost"},{"name":"UsageDate"},{"name":"ServiceName"},{"name":"ResourceLocation"},{"name":"Currency"}],"rows":[[7,20261001,"Storage","EU West","EUR"]]}}`)
	}))
	defer arm.Close()
	a := &CostAPI{HTTP: NoRedirects(http.DefaultClient), ARM: arm.URL}
	b, err := a.usage(context.Background(), "tok", spSub, time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC), time.Date(2026, 10, 2, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b.Payload), `"billed":7,"effective":7`) {
		t.Errorf("effective must fall back to billed: %s", b.Payload)
	}
}

func TestRefusedBilledQueryIsExplainedToTheCustomer(t *testing.T) {
	arm := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, `{"error":{"code":"BadRequest","message":"x"}}`)
	}))
	defer arm.Close()
	a := &CostAPI{HTTP: NoRedirects(http.DefaultClient), ARM: arm.URL}
	_, err := a.usage(context.Background(), "tok", spSub, time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC), time.Date(2026, 10, 2, 0, 0, 0, 0, time.UTC))
	var pe domain.PublicError
	if !errors.As(err, &pe) || !strings.Contains(pe.Message, "BadRequest") {
		t.Errorf("err = %v", err)
	}
}

func TestServerErrorsAreShownWithTheirStatus(t *testing.T) {
	arm := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer arm.Close()
	a := &CostAPI{HTTP: NoRedirects(http.DefaultClient), ARM: arm.URL}
	_, err := a.query(context.Background(), "tok", spSub, "ActualCost", time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC), time.Date(2026, 10, 2, 0, 0, 0, 0, time.UTC), nil)
	var pe domain.PublicError
	if !errors.As(err, &pe) || !strings.Contains(pe.Message, "502") {
		t.Errorf("err = %v", err)
	}
}

func TestServicePrincipalWrongSecretIsAccessDenied(t *testing.T) {
	f := newFakeAzure(t)
	p := f.provider()
	p.OpenSecret = func(domain.Connection) (string, error) { return "not-the-secret", nil }
	if err := p.Validate(context.Background(), spConn()); !errors.Is(err, domain.ErrAccessDenied) {
		t.Errorf("a refused secret must read as access denied: %v", err)
	}
}

func TestNextLinkOutsideTheManagementHostIsRefused(t *testing.T) {
	evil := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { t.Error("the token must never reach another host") }))
	defer evil.Close()
	arm := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `{"properties":{"nextLink":%q,"columns":[{"name":"Cost"},{"name":"UsageDate"}],"rows":[[1,20261001]]}}`, evil.URL+"/steal")
	}))
	defer arm.Close()
	a := &CostAPI{HTTP: NoRedirects(http.DefaultClient), ARM: arm.URL}
	_, err := a.query(context.Background(), "tok", spSub, "ActualCost", time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC), time.Date(2026, 10, 2, 0, 0, 0, 0, time.UTC), nil)
	if err == nil || !strings.Contains(err.Error(), "outside the management endpoint") {
		t.Errorf("err = %v", err)
	}
}

func TestExportModeNeedsThePlatformApplication(t *testing.T) {
	p := &Provider{} // no platform application configured
	c := conn()      // an azblob:// reference
	if err := p.Validate(context.Background(), c); err == nil {
		t.Error("export mode without the platform application must say so")
	}
}

func TestThrottleWaitReadsTheCostManagementHeaders(t *testing.T) {
	h := http.Header{}
	if got := throttleWait(h); got != 15*time.Second {
		t.Errorf("no header: %v", got)
	}
	h.Set("x-ms-ratelimit-microsoft.costmanagement-entity-retry-after", "42")
	h.Set("Retry-After", "7")
	if got := throttleWait(h); got != 42*time.Second {
		t.Errorf("the longest wait must win: %v", got)
	}
}

func TestWindowsAreChunked(t *testing.T) {
	from := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	ws := windows(from, from.AddDate(0, 0, 200))
	if len(ws) != 3 || !ws[0][1].Equal(ws[1][0]) || !ws[2][1].Equal(from.AddDate(0, 0, 200)) {
		t.Errorf("windows: %v", ws)
	}
}
