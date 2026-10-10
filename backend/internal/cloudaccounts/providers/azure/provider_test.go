package azure

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/quixgit/greenops-reliabilix/backend/internal/cloudaccounts/domain"
)

const (
	tenant = "99999999-8888-7777-6666-555555555555"
	sub    = "11111111-2222-3333-4444-555555555555"
	other  = "aaaaaaaa-0000-0000-0000-000000000000"
	header = "BilledCost,EffectiveCost,BillingCurrency,ChargePeriodStart,RegionId,ServiceName,ServiceCategory,ConsumedQuantity,ConsumedUnit,SubAccountId,ChargeCategory,x_SkuMeterCategory,x_SkuMeterSubcategory,x_SkuMeterName,x_SkuCoreCount\n"
)

type fakeBlobs struct {
	meta  map[string]string
	files map[string][]byte
	err   error
	opens []string
}

func (f *fakeBlobs) ContainerMetadata(context.Context, domain.AzureExport) (map[string]string, error) {
	return f.meta, f.err
}

func (f *fakeBlobs) List(_ context.Context, _ domain.AzureExport, prefix string) ([]BlobInfo, error) {
	var out []BlobInfo
	for n, b := range f.files {
		if strings.HasPrefix(n, prefix) {
			out = append(out, BlobInfo{n, int64(len(b))})
		}
	}
	return out, f.err
}

func (f *fakeBlobs) Open(_ context.Context, _ domain.AzureExport, name string) (io.ReadCloser, int64, error) {
	f.opens = append(f.opens, name)
	b, ok := f.files[name]
	if !ok {
		return nil, 0, domain.ErrExportNotFound
	}
	return io.NopCloser(bytes.NewReader(b)), int64(len(b)), nil
}

func conn() domain.Connection {
	return domain.Connection{Provider: domain.Azure, AccountRef: sub, ExternalID: "rlx-abc",
		CredentialRef: "azblob://" + tenant + "@acmebilling/exports/rlx/daily"}
}

func owned() map[string]string { return map[string]string{"rlx_abc": "1"} }

func gz(s string) []byte {
	var b bytes.Buffer
	w := gzip.NewWriter(&b)
	_, _ = w.Write([]byte(s))
	_ = w.Close()
	return b.Bytes()
}

var (
	from = time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	to   = time.Date(2026, 10, 4, 0, 0, 0, 0, time.UTC)
)

func row(day, region, service, cat, qty, unit, subscription, charge, meterCat, meterSub, meterName, cores, billed string) string {
	return fmt.Sprintf("%s,%s,EUR,%sT00:00:00Z,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s\n",
		billed, billed, day, region, service, cat, qty, unit, subscription, charge, meterCat, meterSub, meterName, cores)
}

func usage(t *testing.T, p *Provider) map[string]any {
	t.Helper()
	batches, err := p.GetUsage(context.Background(), domain.UsageRequest{Connection: conn(), From: from, To: to})
	if err != nil {
		t.Fatal(err)
	}
	if len(batches) != 1 || batches[0].Provider != "azure" || batches[0].Source != SourceFocusExport {
		t.Fatalf("batches: %+v", batches)
	}
	var got map[string]any
	if err := json.Unmarshal(batches[0].Payload, &got); err != nil {
		t.Fatal(err)
	}
	return got
}

func TestGetUsageAggregatesMeasuredAndCostLines(t *testing.T) {
	csvData := header +
		// two instances of a 4-core size, running the whole day: 2 x 24 h x 4 vCPU
		row("2026-10-01", "westeurope", "Virtual Machines", "Compute", "24", "1 Hour", sub, "Usage", "Virtual Machines", "Dsv3 Series", "D4s v3", "4", "10") +
		row("2026-10-01", "westeurope", "Virtual Machines", "Compute", "24", "1 Hour", sub, "Usage", "Virtual Machines", "Dsv3 Series", "D4s v3", "4", "5") +
		// the Windows licence meter repeats the same hours: not counted again, and it stays a cost-based line
		row("2026-10-01", "westeurope", "Virtual Machines", "Compute", "24", "1 Hour", sub, "Usage", "Virtual Machines", "Dsv3 Series Windows", "D4s v3", "4", "3") +
		// a reservation purchase is a fee, not energy
		row("2026-10-01", "", "Virtual Machines", "Compute", "1", "1", sub, "Purchase", "Virtual Machines", "", "", "", "100") +
		// blob storage, billed per 100 GB/month
		row("2026-10-02", "northeurope", "Storage", "Storage", "3", "100 GB/Month", sub, "Usage", "Storage", "General Block Blob v2", "Hot LRS Data Stored", "", "6") +
		row("2026-10-02", "northeurope", "Storage", "Storage", "1000", "10K", sub, "Usage", "Storage", "General Block Blob v2", "Hot LRS Write Operations", "", "1") +
		// another subscription of the same billing scope, and a day outside the window
		row("2026-10-01", "westeurope", "Virtual Machines", "Compute", "24", "1 Hour", other, "Usage", "Virtual Machines", "Dsv3 Series", "D4s v3", "4", "999") +
		row("2026-09-30", "westeurope", "Virtual Machines", "Compute", "24", "1 Hour", sub, "Usage", "Virtual Machines", "Dsv3 Series", "D4s v3", "4", "888")
	blobs := &fakeBlobs{meta: owned(), files: map[string][]byte{
		"rlx/daily/20261001-20261031/1759300000/aaaa/part_0_0001.csv.gz": gz(csvData),
		"rlx/daily/20261001-20261031/1759300000/aaaa/manifest.json":      []byte("{}"),
	}}
	got := usage(t, &Provider{Blobs: blobs})

	cost := got["cost"].([]any)
	byKey := map[string][2]float64{}
	for _, r := range cost {
		m := r.(map[string]any)
		byKey[m["day"].(string)+"|"+m["service"].(string)+"|"+m["region"].(string)] = [2]float64{m["billed"].(float64), m["effective"].(float64)}
	}
	if v := byKey["2026-10-01|Virtual Machines - Compute|westeurope"]; v[0] != 15 {
		t.Errorf("measured VM cost line: %v (all: %v)", v, byKey)
	}
	if v := byKey["2026-10-01|Virtual Machines - Other|westeurope"]; v[0] != 3 {
		t.Errorf("licence line must stay a separate, unmeasured line: %v", v)
	}
	if v := byKey["2026-10-01|Virtual Machines - Other|"]; v[0] != 100 {
		t.Errorf("purchase line: %v", v)
	}
	if v := byKey["2026-10-02|Storage - Capacity|northeurope"]; v[0] != 6 {
		t.Errorf("measured storage cost line: %v", v)
	}
	if v := byKey["2026-10-02|Storage|northeurope"]; v[0] != 1 {
		t.Errorf("operations line: %v", v)
	}
	for k, v := range byKey {
		if v[0] == 999 || v[0] == 888 {
			t.Errorf("a row of another subscription or outside the window leaked: %s %v", k, v)
		}
	}

	vcpu := got["vm_vcpu_hours"].([]any)
	if len(vcpu) != 1 || vcpu[0].(map[string]any)["value"].(float64) != 192 { // (24+24) h x 4 vCPU
		t.Errorf("vCPU hours: %v", vcpu)
	}
	st := got["storage_gb_months"].([]any)
	if len(st) != 1 || st[0].(map[string]any)["value"].(float64) != 300 {
		t.Errorf("storage GB-months: %v", st)
	}
}

func TestOnlyTheNewestRunOfARangeIsRead(t *testing.T) {
	old := header + row("2026-10-01", "westeurope", "Virtual Machines", "Compute", "1", "1 Hour", sub, "Usage", "Virtual Machines", "D", "D", "2", "1")
	newer := header + row("2026-10-01", "westeurope", "Virtual Machines", "Compute", "5", "1 Hour", sub, "Usage", "Virtual Machines", "D", "D", "2", "1")
	blobs := &fakeBlobs{meta: owned(), files: map[string][]byte{
		"rlx/daily/20261001-20261031/1759200000/g1/part_0_0001.csv": []byte(old),
		"rlx/daily/20261001-20261031/1759300000/g2/part_0_0001.csv": []byte(newer),
		"rlx/daily/20250901-20250930/1/g/part_0_0001.csv":           []byte(old), // a range outside the window
		"elsewhere/20261001-20261031/9/g/part_0_0001.csv":           []byte(old), // outside the export directory
	}}
	got := usage(t, &Provider{Blobs: blobs})
	if v := got["vm_vcpu_hours"].([]any)[0].(map[string]any)["value"].(float64); v != 10 {
		t.Errorf("vCPU hours = %v, want 10 (5 h x 2 cores of the newest run only)", v)
	}
	if len(blobs.opens) != 1 || !strings.Contains(blobs.opens[0], "1759300000") {
		t.Errorf("opened %v", blobs.opens)
	}
}

func TestOwnershipIsRequired(t *testing.T) {
	files := map[string][]byte{"rlx/daily/20261001-20261031/1/g/part_0_0001.csv": []byte(header)}
	for name, meta := range map[string]map[string]string{"no metadata": {}, "another token": {"rlx_zzz": "1"}, "wrong value": {"rlx_abc": "0"}} {
		p := &Provider{Blobs: &fakeBlobs{meta: meta, files: files}}
		if err := p.Validate(context.Background(), conn()); !errors.Is(err, domain.ErrOwnershipNotProven) {
			t.Errorf("%s: Validate: %v", name, err)
		}
		if _, err := p.GetUsage(context.Background(), domain.UsageRequest{Connection: conn(), From: from, To: to}); !errors.Is(err, domain.ErrOwnershipNotProven) {
			t.Errorf("%s: GetUsage: %v", name, err)
		}
	}
}

func TestValidate(t *testing.T) {
	ok := &Provider{Blobs: &fakeBlobs{meta: owned(), files: map[string][]byte{"rlx/daily/20261001-20261031/1/g/part_0_0001.csv": []byte(header)}}}
	if err := ok.Validate(context.Background(), conn()); err != nil {
		t.Errorf("valid export: %v", err)
	}
	empty := &Provider{Blobs: &fakeBlobs{meta: owned(), files: map[string][]byte{}}}
	if err := empty.Validate(context.Background(), conn()); !errors.Is(err, domain.ErrExportNotFound) {
		t.Errorf("nothing delivered yet: %v", err)
	}
	denied := &Provider{Blobs: &fakeBlobs{err: domain.ErrAccessDenied}}
	if err := denied.Validate(context.Background(), conn()); !errors.Is(err, domain.ErrAccessDenied) {
		t.Errorf("access denied: %v", err)
	}
}

func TestExportOfAnotherSubscriptionIsRefused(t *testing.T) {
	csvData := header + row("2026-10-01", "westeurope", "Storage", "Storage", "1", "1 GB/Month", other, "Usage", "Storage", "x", "Hot Data Stored", "", "1")
	p := &Provider{Blobs: &fakeBlobs{meta: owned(), files: map[string][]byte{"rlx/daily/20261001-20261031/1/g/part_0_0001.csv": []byte(csvData)}}}
	_, err := p.GetUsage(context.Background(), domain.UsageRequest{Connection: conn(), From: from, To: to})
	if !errors.Is(err, domain.ErrSubscriptionNotInExport) {
		t.Errorf("err = %v", err)
	}
}

func TestNotAFocusExport(t *testing.T) {
	p := &Provider{Blobs: &fakeBlobs{meta: owned(), files: map[string][]byte{
		"rlx/daily/20261001-20261031/1/g/part_0_0001.csv": []byte("Date,CostInBillingCurrency\n2026-10-01,1\n")}}}
	_, err := p.GetUsage(context.Background(), domain.UsageRequest{Connection: conn(), From: from, To: to})
	if err == nil || !strings.Contains(err.Error(), "FOCUS") {
		t.Errorf("a non-FOCUS export must say so: %v", err)
	}
}

func TestExportBoundsAreEnforced(t *testing.T) {
	// a header-less file of one endless line must fail instead of being buffered whole
	p := &Provider{Blobs: &fakeBlobs{meta: owned(), files: map[string][]byte{
		"rlx/daily/20261001-20261031/1/g/part_0_0001.csv": bytes.Repeat([]byte("a"), maxLineBytes+10)}}}
	if _, err := p.GetUsage(context.Background(), domain.UsageRequest{Connection: conn(), From: from, To: to}); err == nil {
		t.Error("an over-long line must be rejected")
	}
}

func TestRESTBlobsAndTokens(t *testing.T) {
	var tokenCalls int
	idp := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tokenCalls++
		if !strings.HasPrefix(r.URL.Path, "/"+tenant+"/oauth2/v2.0/token") {
			t.Errorf("token path %s", r.URL.Path)
		}
		_ = r.ParseForm()
		if r.Form.Get("client_secret") != "s3cret" || r.Form.Get("scope") != "https://storage.azure.com/.default" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		_, _ = w.Write([]byte(`{"access_token":"tok","expires_in":3600}`))
	}))
	defer idp.Close()
	store := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer tok" || r.Header.Get("x-ms-version") == "" {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		q := r.URL.Query()
		switch {
		case q.Get("comp") == "list" && q.Get("marker") == "":
			_, _ = w.Write([]byte(`<EnumerationResults><Blobs><Blob><Name>rlx/daily/20261001-20261031/1/g/part_0_0001.csv</Name><Properties><Content-Length>7</Content-Length></Properties></Blob></Blobs><NextMarker>m2</NextMarker></EnumerationResults>`))
		case q.Get("comp") == "list":
			_, _ = w.Write([]byte(`<EnumerationResults><Blobs><Blob><Name>rlx/daily/20261001-20261031/1/g/part_0_0002.csv</Name><Properties><Content-Length>9</Content-Length></Properties></Blob></Blobs><NextMarker/></EnumerationResults>`))
		case q.Get("restype") == "container":
			w.Header().Set("x-ms-meta-rlx_abc", "1")
		default:
			if r.URL.Path != "/exports/rlx/daily/20261001-20261031/1/g/part_0_0001.csv" {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			_, _ = w.Write([]byte("a,b\n1,2\n"))
		}
	}))
	defer store.Close()

	h := NoRedirects(http.DefaultClient)
	tokens := &ClientCredentials{ClientID: "app", Secret: "s3cret", Authority: idp.URL, HTTP: h}
	b := &RESTBlobs{HTTP: h, Tokens: tokens, Endpoint: func(string) string { return store.URL }}
	e := domain.AzureExport{TenantID: tenant, Account: "acmebilling", Container: "exports", Dir: "rlx/daily"}
	ctx := context.Background()

	meta, err := b.ContainerMetadata(ctx, e)
	if err != nil || meta["rlx_abc"] != "1" {
		t.Fatalf("metadata %v %v", meta, err)
	}
	list, err := b.List(ctx, e, "rlx/daily/")
	if err != nil || len(list) != 2 || list[1].Size != 9 {
		t.Fatalf("list (two pages) %v %v", list, err)
	}
	body, _, err := b.Open(ctx, e, list[0].Name)
	if err != nil {
		t.Fatal(err)
	}
	data, _ := io.ReadAll(body)
	_ = body.Close()
	if string(data) != "a,b\n1,2\n" {
		t.Errorf("blob = %q", data)
	}
	if _, _, err := b.Open(ctx, e, "rlx/daily/missing.csv"); !errors.Is(err, domain.ErrExportNotFound) {
		t.Errorf("a missing blob: %v", err)
	}
	if tokenCalls != 1 {
		t.Errorf("the token must be cached: %d requests", tokenCalls)
	}

	bad := &ClientCredentials{ClientID: "app", Secret: "wrong", Authority: idp.URL, HTTP: h}
	if _, err := bad.Token(ctx, tenant); !errors.Is(err, domain.ErrAccessDenied) {
		t.Errorf("a refused application must read as access denied: %v", err)
	}
}
