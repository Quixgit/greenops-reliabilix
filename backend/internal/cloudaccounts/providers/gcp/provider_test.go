package gcp

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/quixgit/greenops-reliabilix/backend/internal/cloudaccounts/domain"
)

const extID = "rlx-0123456789abcdef0123456789abcdef01234567"

func conn() domain.Connection {
	return domain.Connection{Provider: domain.GCP, AccountRef: "acme-prod-123", ExternalID: extID,
		CredentialRef: "bq://acme-billing-1/billing_export/gcp_billing_export_v1_AAAAAA_BBBBBB_CCCCCC"}
}

// fakeBigQuery is an httptest BigQuery: datasets.get, jobs.query and getQueryResults.
type fakeBigQuery struct {
	mu          sync.Mutex
	labels      map[string]string
	datasetCode int    // non-zero: datasets.get fails with this status
	datasetBody string // error body for datasets.get
	queryCode   int
	queryBody   string
	pages       [][][]string // result pages; page i is served on the i-th request
	incomplete  int          // number of "jobComplete:false" answers before results
	requests    []recorded
}

type recorded struct {
	Method, Path string
	Body         map[string]any
}

func (f *fakeBigQuery) handler(t *testing.T) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		raw, _ := io.ReadAll(r.Body)
		var body map[string]any
		_ = json.Unmarshal(raw, &body)
		f.requests = append(f.requests, recorded{r.Method, r.URL.Path, body})
		switch {
		case strings.Contains(r.URL.Path, "/datasets/"):
			if f.datasetCode != 0 {
				w.WriteHeader(f.datasetCode)
				_, _ = w.Write([]byte(f.datasetBody))
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"labels": f.labels})
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/queries"):
			if f.queryCode != 0 {
				w.WriteHeader(f.queryCode)
				_, _ = w.Write([]byte(f.queryBody))
				return
			}
			if body["dryRun"] == true {
				_, _ = w.Write([]byte(`{"jobComplete":true}`))
				return
			}
			f.serve(w, 0)
		default: // getQueryResults
			page := 0
			if tok := r.URL.Query().Get("pageToken"); tok != "" {
				page, _ = strconv.Atoi(tok)
			}
			f.serve(w, page)
		}
	})
}

func (f *fakeBigQuery) serve(w http.ResponseWriter, page int) {
	if f.incomplete > 0 {
		f.incomplete--
		_, _ = w.Write([]byte(`{"jobComplete":false,"jobReference":{"jobId":"j1","location":"EU"}}`))
		return
	}
	rows := []map[string]any{}
	if page < len(f.pages) {
		for _, r := range f.pages[page] {
			cells := []map[string]any{}
			for _, v := range r {
				cells = append(cells, map[string]any{"v": v})
			}
			rows = append(rows, map[string]any{"f": cells})
		}
	}
	out := map[string]any{"jobComplete": true, "jobReference": map[string]string{"jobId": "j1", "location": "EU"}, "rows": rows}
	if page+1 < len(f.pages) {
		out["pageToken"] = strconv.Itoa(page + 1)
	}
	_ = json.NewEncoder(w).Encode(out)
}

func setup(t *testing.T, f *fakeBigQuery) *Provider {
	t.Helper()
	srv := httptest.NewServer(f.handler(t))
	t.Cleanup(srv.Close)
	return &Provider{BQ: &RESTClient{HTTP: srv.Client(), Base: srv.URL}, JobProject: "platform-proj"}
}

func row(day, service, sku, region string) []string {
	return []string{day, service, sku, region, "USD", "hour", "10.5", "-1.5", "24"}
}

func usageReq() domain.UsageRequest {
	from := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	return domain.UsageRequest{Connection: conn(), From: from, To: from.AddDate(0, 0, 7)}
}

func TestGetUsageQueriesWithParametersAndReturnsAggregate(t *testing.T) {
	f := &fakeBigQuery{labels: map[string]string{extID: "1"}, pages: [][][]string{
		{row("2026-09-01", "Compute Engine", "N2 Instance Core running in Frankfurt", "europe-west3")},
		{row("2026-09-02", "Cloud Storage", "Standard Storage Frankfurt", "europe-west3")},
	}, incomplete: 1}
	batches, err := setup(t, f).GetUsage(context.Background(), usageReq())
	if err != nil || len(batches) != 1 || batches[0].Source != SourceBigQuery || batches[0].Provider != "gcp" {
		t.Fatalf("batches=%v err=%v", batches, err)
	}
	var out struct{ Rows []map[string]string }
	if err := json.Unmarshal(batches[0].Payload, &out); err != nil || len(out.Rows) != 2 {
		t.Fatalf("payload: %v %s", err, batches[0].Payload)
	}
	if out.Rows[0]["service"] != "Compute Engine" || out.Rows[0]["credits"] != "-1.5" || out.Rows[1]["day"] != "2026-09-02" {
		t.Errorf("rows wrong (pagination and polling must both work): %+v", out.Rows)
	}

	var q map[string]any
	for _, r := range f.requests {
		if r.Method == http.MethodPost {
			q = r.Body
			if !strings.Contains(r.Path, "/projects/platform-proj/queries") {
				t.Errorf("the job must run in the platform's own project, got %s", r.Path)
			}
		}
	}
	sql, _ := q["query"].(string)
	if !strings.Contains(sql, "`acme-billing-1.billing_export.gcp_billing_export_v1_AAAAAA_BBBBBB_CCCCCC`") || !strings.Contains(sql, "project.id = @project_id") {
		t.Errorf("query must read only the validated table and filter by project:\n%s", sql)
	}
	if strings.Contains(sql, "acme-prod-123") {
		t.Errorf("the project id must be a parameter, never part of the SQL text:\n%s", sql)
	}
	if q["maximumBytesBilled"] != "21474836480" || q["useLegacySql"] != false {
		t.Errorf("the query must carry a byte cap and use standard SQL: %v", q)
	}
	params, _ := q["queryParameters"].([]any)
	if len(params) != 3 {
		t.Errorf("want 3 parameters, got %v", q["queryParameters"])
	}
}

func TestOwnershipLabelIsRequiredBeforeAnyQuery(t *testing.T) {
	for name, labels := range map[string]map[string]string{
		"no labels":       nil,
		"someone else's":  {"rlx-ffffffffffffffffffffffffffffffffffffffff": "1"},
		"wrong value":     {extID: "0"},
		"label for other": {"owner": extID},
	} {
		f := &fakeBigQuery{labels: labels}
		_, err := setup(t, f).GetUsage(context.Background(), usageReq())
		if !errors.Is(err, domain.ErrOwnershipNotProven) {
			t.Errorf("%s: want ErrOwnershipNotProven, got %v", name, err)
		}
		for _, r := range f.requests {
			if r.Method == http.MethodPost {
				t.Errorf("%s: no query may run before ownership is proven", name)
			}
		}
	}
	c := conn()
	c.ExternalID = ""
	if err := setup(t, &fakeBigQuery{labels: map[string]string{"": "1"}}).Validate(context.Background(), c); !errors.Is(err, domain.ErrOwnershipNotProven) {
		t.Errorf("an empty external id must never match: %v", err)
	}
}

func TestCustomerSideFailuresAreTranslated(t *testing.T) {
	disabled := `{"error":{"code":403,"errors":[{"reason":"accessNotConfigured"}]}}`
	disabledNew := `{"error":{"code":403,"status":"PERMISSION_DENIED","details":[{"reason":"SERVICE_DISABLED"}]}}`
	cases := []struct {
		name string
		f    *fakeBigQuery
		want error
	}{
		{"BigQuery API disabled (classic error)", &fakeBigQuery{datasetCode: 403, datasetBody: disabled}, domain.ErrBigQueryDisabled},
		{"BigQuery API disabled (ErrorInfo)", &fakeBigQuery{datasetCode: 403, datasetBody: disabledNew}, domain.ErrBigQueryDisabled},
		{"no dataset: BigQuery never set up", &fakeBigQuery{datasetCode: 404, datasetBody: `{"error":{"code":404}}`}, domain.ErrExportNotFound},
		{"no read access", &fakeBigQuery{datasetCode: 403, datasetBody: `{"error":{"code":403,"errors":[{"reason":"accessDenied"}]}}`}, domain.ErrAccessDenied},
		{"table missing: export not delivered yet", &fakeBigQuery{labels: map[string]string{extID: "1"}, queryCode: 404, queryBody: `{"error":{"code":404,"errors":[{"reason":"notFound"}]}}`}, domain.ErrExportNotFound},
		{"no access to the table", &fakeBigQuery{labels: map[string]string{extID: "1"}, queryCode: 403, queryBody: `{"error":{"code":403,"errors":[{"reason":"accessDenied"}]}}`}, domain.ErrAccessDenied},
	}
	for _, c := range cases {
		if _, err := setup(t, c.f).GetUsage(context.Background(), usageReq()); !errors.Is(err, c.want) {
			t.Errorf("%s: got %v, want %v", c.name, err, c.want)
		}
		if err := setup(t, c.f).Validate(context.Background(), conn()); !errors.Is(err, c.want) {
			t.Errorf("%s (validate): got %v, want %v", c.name, err, c.want)
		}
		if !domain.NeedsCustomerAction(c.want) {
			t.Errorf("%v must be reported as needing customer action", c.want)
		}
	}
}

func TestDisabledApiInThePlatformProjectIsNotBlamedOnTheCustomer(t *testing.T) {
	f := &fakeBigQuery{labels: map[string]string{extID: "1"}, queryCode: 403, queryBody: `{"error":{"code":403,"errors":[{"reason":"accessNotConfigured"}]}}`}
	_, err := setup(t, f).GetUsage(context.Background(), usageReq())
	if err == nil || domain.NeedsCustomerAction(err) || !strings.Contains(err.Error(), "platform's job project") {
		t.Fatalf("a disabled API in the job project is an operator problem, got %v", err)
	}
}

func TestValidateDryRunsAndReadsNothing(t *testing.T) {
	f := &fakeBigQuery{labels: map[string]string{extID: "1"}}
	if err := setup(t, f).Validate(context.Background(), conn()); err != nil {
		t.Fatal(err)
	}
	var dry bool
	for _, r := range f.requests {
		if r.Method == http.MethodPost {
			dry = r.Body["dryRun"] == true
		}
	}
	if !dry {
		t.Error("validation must be a dry run: it checks access and schema without scanning or billing")
	}
}

func TestInvalidReferencesNeverReachBigQuery(t *testing.T) {
	for _, ref := range []string{
		"bq://acme/ds/some_other_table",
		"bq://acme-billing-1/ds`/gcp_billing_export_v1_X",
		"bq://acme-billing-1/ds/gcp_billing_export_v1_X; DROP TABLE x",
		"bq://acme-billing-1/ds",
		"https://evil/ds/gcp_billing_export_v1_X",
		"bq://Bad_Project/ds/gcp_billing_export_v1_X",
	} {
		f := &fakeBigQuery{labels: map[string]string{extID: "1"}}
		c := conn()
		c.CredentialRef = ref
		if _, err := setup(t, f).GetUsage(context.Background(), domain.UsageRequest{Connection: c, From: time.Now(), To: time.Now()}); !errors.Is(err, domain.ErrInvalidConnection) {
			t.Errorf("%q: want ErrInvalidConnection, got %v", ref, err)
		}
		if len(f.requests) != 0 {
			t.Errorf("%q: no request may be sent for an invalid reference", ref)
		}
	}
}

func TestRowCapAndResultShape(t *testing.T) {
	big := make([][]string, 6)
	for i := range big {
		big[i] = row("2026-09-01", "S", "sku", "r")
	}
	f := &fakeBigQuery{labels: map[string]string{extID: "1"}, pages: [][][]string{big}}
	p := setup(t, f)
	p.MaxRows = 5
	if _, err := p.GetUsage(context.Background(), usageReq()); err == nil || !strings.Contains(err.Error(), "too many") {
		t.Errorf("an oversized result must be rejected, got %v", err)
	}
	f = &fakeBigQuery{labels: map[string]string{extID: "1"}, pages: [][][]string{{{"only", "three", "cols"}}}}
	if _, err := setup(t, f).GetUsage(context.Background(), usageReq()); err == nil || !strings.Contains(err.Error(), "shape") {
		t.Errorf("a changed schema must be detected, got %v", err)
	}
}

func TestByteCapCanBeConfigured(t *testing.T) {
	f := &fakeBigQuery{labels: map[string]string{extID: "1"}}
	p := setup(t, f)
	p.MaxBytesBilled = 1 << 20
	if _, err := p.GetUsage(context.Background(), usageReq()); err != nil {
		t.Fatal(err)
	}
	for _, r := range f.requests {
		if r.Method == http.MethodPost && r.Body["maximumBytesBilled"] != "1048576" {
			t.Errorf("cap not applied: %v", r.Body["maximumBytesBilled"])
		}
	}
}

func TestPreflight(t *testing.T) {
	f := &fakeBigQuery{}
	if err := setup(t, f).Preflight(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(f.requests) != 1 || f.requests[0].Body["dryRun"] != true {
		t.Fatalf("preflight must be a single dry run: %+v", f.requests)
	}
	off := &fakeBigQuery{queryCode: 403, queryBody: `{"error":{"code":403,"errors":[{"reason":"accessNotConfigured"}]}}`}
	if err := setup(t, off).Preflight(context.Background()); err == nil || !strings.Contains(err.Error(), "job project") {
		t.Fatalf("a disabled API must be reported as a platform problem: %v", err)
	}
}
