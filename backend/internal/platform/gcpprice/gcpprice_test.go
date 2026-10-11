package gcpprice

import (
	"context"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const catalog = `{"skus":[
 {"description":"N2 Instance Core running in Americas","category":{"resourceFamily":"Compute","usageType":"OnDemand"},"serviceRegions":["us-central1"],"pricingInfo":[{"pricingExpression":{"tieredRates":[{"unitPrice":{"units":"0","nanos":31611000}}]}}]},
 {"description":"N2 Instance Ram running in Americas","category":{"resourceFamily":"Compute","usageType":"OnDemand"},"serviceRegions":["us-central1"],"pricingInfo":[{"pricingExpression":{"tieredRates":[{"unitPrice":{"units":"0","nanos":4237000}}]}}]},
 {"description":"N2 Instance Core running in Frankfurt","category":{"resourceFamily":"Compute","usageType":"OnDemand"},"serviceRegions":["europe-west3"],"pricingInfo":[{"pricingExpression":{"tieredRates":[{"unitPrice":{"units":"0","nanos":41094000}}]}}]},
 {"description":"N2 Instance Ram running in Frankfurt","category":{"resourceFamily":"Compute","usageType":"OnDemand"},"serviceRegions":["europe-west3"],"pricingInfo":[{"pricingExpression":{"tieredRates":[{"unitPrice":{"units":"0","nanos":5508000}}]}}]},
 {"description":"Spot Preemptible N2 Instance Core running in Frankfurt","category":{"resourceFamily":"Compute","usageType":"Preemptible"},"serviceRegions":["europe-west3"],"pricingInfo":[{"pricingExpression":{"tieredRates":[{"unitPrice":{"units":"0","nanos":1000000}}]}}]}
]}`

func TestIndexIsTheRatioOfN2Standard4(t *testing.T) {
	var key string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key = r.URL.Query().Get("key")
		if !strings.Contains(r.URL.Path, "/v1/services/6F81-5844-456A/skus") {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_, _ = w.Write([]byte(catalog))
	}))
	defer srv.Close()
	c := New("k-123")
	c.BaseURL = srv.URL
	idx, errs, err := c.BuildIndex(context.Background(), []string{"us-central1", "europe-west3", "asia-east1"})
	if err != nil {
		t.Fatal(err)
	}
	if key != "k-123" || len(errs) != 1 {
		t.Fatalf("key %q errs %v", key, errs)
	}
	m := map[string]float64{}
	for _, i := range idx {
		m[i.Region] = i.Idx
	}
	base := 4*0.031611 + 16*0.004237
	fr := 4*0.041094 + 16*0.005508
	if m["us-central1"] != 1 || math.Abs(m["europe-west3"]-math.Round(fr/base*10000)/10000) > 1e-9 {
		t.Fatalf("index %v", m)
	}
}

func TestNoKeyIsNotAGuess(t *testing.T) {
	if _, _, err := New("").BuildIndex(context.Background(), []string{"us-central1"}); err == nil {
		t.Fatal("no key must fail")
	}
}

func TestKeyIsNeverInAnError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusForbidden) }))
	defer srv.Close()
	c := New("secret-key-xyz")
	c.BaseURL = srv.URL
	if err := c.Check(context.Background()); err == nil || strings.Contains(err.Error(), "secret-key-xyz") {
		t.Fatalf("%v", err)
	}
}

const sqlCatalog = `{"skus":[
 {"description":"Cloud SQL for PostgreSQL: Zonal - vCPU in Iowa","category":{"usageType":"OnDemand"},"serviceRegions":["us-central1"],"pricingInfo":[{"pricingExpression":{"tieredRates":[{"unitPrice":{"units":"0","nanos":41300000}}]}}]},
 {"description":"Cloud SQL for PostgreSQL: Zonal - RAM in Iowa","category":{"usageType":"OnDemand"},"serviceRegions":["us-central1"],"pricingInfo":[{"pricingExpression":{"tieredRates":[{"unitPrice":{"units":"0","nanos":7000000}}]}}]},
 {"description":"Cloud SQL for PostgreSQL: Regional - vCPU in Iowa","category":{"usageType":"OnDemand"},"serviceRegions":["us-central1"],"pricingInfo":[{"pricingExpression":{"tieredRates":[{"unitPrice":{"units":"0","nanos":82600000}}]}}]},
 {"description":"Cloud SQL for PostgreSQL: Zonal - vCPU in Frankfurt","category":{"usageType":"OnDemand"},"serviceRegions":["europe-west3"],"pricingInfo":[{"pricingExpression":{"tieredRates":[{"unitPrice":{"units":"0","nanos":53700000}}]}}]},
 {"description":"Cloud SQL for PostgreSQL: Zonal - RAM in Frankfurt","category":{"usageType":"OnDemand"},"serviceRegions":["europe-west3"],"pricingInfo":[{"pricingExpression":{"tieredRates":[{"unitPrice":{"units":"0","nanos":9100000}}]}}]},
 {"description":"Cloud SQL for PostgreSQL: Zonal - Standard storage in Frankfurt","category":{"usageType":"OnDemand"},"serviceRegions":["europe-west3"],"pricingInfo":[{"pricingExpression":{"tieredRates":[{"unitPrice":{"units":"0","nanos":220000000}}]}}]}
]}`

func TestCloudSQLUsesTheZonalVCPUAndRAMOnly(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "/v1/services/9662-B51E-5089/skus") {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_, _ = w.Write([]byte(sqlCatalog))
	}))
	defer srv.Close()
	c := New("k-123")
	c.BaseURL = srv.URL
	idx, _, err := c.BuildFamilyIndex(context.Background(), CloudSQL, []string{"us-central1", "europe-west3"})
	if err != nil {
		t.Fatal(err)
	}
	m := map[string]float64{}
	for _, i := range idx {
		m[i.Region] = i.Idx
	}
	base := 4*0.0413 + 16*0.007
	fr := 4*0.0537 + 16*0.0091
	if m["us-central1"] != 1 || math.Abs(m["europe-west3"]-math.Round(fr/base*10000)/10000) > 1e-9 {
		t.Fatalf("index %v (storage and regional vCPU must not count)", m)
	}
}
