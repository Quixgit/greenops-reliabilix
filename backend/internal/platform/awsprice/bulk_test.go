package awsprice

import (
	"context"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// a miniature of a bulk file: products first, then terms, with the variants the real file has
func bulkFile(k float64) string {
	prod := func(sku, family, extra string) string {
		return fmt.Sprintf(`%q:{"sku":%q,"productFamily":%q,"attributes":{%s}}`, sku, sku, family, extra)
	}
	term := func(sku string, usd float64) string {
		return fmt.Sprintf(`%q:{"%s.JRTCKXETXF":{"priceDimensions":{"%s.JRTCKXETXF.6YS6EN2CT7":{"unit":"Hrs","pricePerUnit":{"USD":"%.4f"}}}}}`, sku, sku, sku, usd)
	}
	pg := `"databaseEngine":"PostgreSQL","deploymentOption":"Single-AZ","licenseModel":"No license required","locationType":"AWS Region",`
	products := []string{
		prod("A1", "Database Instance", pg+`"instanceType":"db.m5.large","usagetype":"InstanceUsage:db.m5.large"`),
		prod("A2", "Database Instance", pg+`"instanceType":"db.r5.large","usagetype":"EUC1-InstanceUsage:db.r5.large"`),
		prod("A3", "Database Instance", strings.Replace(pg, "Single-AZ", "Multi-AZ", 1)+`"instanceType":"db.m5.large","usagetype":"Multi-AZUsage:db.m5.large"`),
		prod("A4", "Database Instance", strings.Replace(pg, "PostgreSQL", "MySQL", 1)+`"instanceType":"db.m5.large","usagetype":"InstanceUsage:db.m5.large"`),
		prod("A5", "Database Storage", `"usagetype":"RDS:GP2-Storage"`),
	}
	terms := []string{term("A1", 0.178*k), term("A2", 0.25*k), term("A3", 0.356*k), term("A4", 0.1*k)}
	return `{"formatVersion":"v1.0","products":{` + strings.Join(products, ",") + `},"terms":{"OnDemand":{` + strings.Join(terms, ",") + `},"Reserved":{"A1":{}}}}`
}

func TestStreamPricesKeepsOnlyThePlainPostgresSingleAZNodes(t *testing.T) {
	svc := RDS
	svc.Basket = []string{"db.m5.large", "db.r5.large"}
	got, err := streamPrices(strings.NewReader(bulkFile(1)), svc)
	if err != nil || len(got) != 2 || math.Abs(got["db.m5.large"]-0.178) > 1e-9 || math.Abs(got["db.r5.large"]-0.25) > 1e-9 {
		t.Fatalf("%v %v", got, err)
	}
}

func TestServiceIndexIsTheRatioToTheBaseline(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		k := 1.0
		if strings.Contains(r.URL.Path, "/eu-central-1/") {
			k = 1.2
		}
		if strings.Contains(r.URL.Path, "/sa-east-1/") {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_, _ = w.Write([]byte(bulkFile(k)))
	}))
	defer srv.Close()
	b := NewBulk()
	b.BaseURL = srv.URL
	svc := RDS
	svc.Basket = []string{"db.m5.large", "db.r5.large"}
	svc.MinOverlap = 2
	idx, errs, err := b.BuildServiceIndex(context.Background(), svc, []string{"eu-central-1", "sa-east-1"})
	if err != nil || len(errs) != 1 || errs["sa-east-1"] == nil {
		t.Fatalf("%v %v", err, errs)
	}
	m := map[string]float64{}
	for _, i := range idx {
		m[i.Region] = i.Idx
	}
	if m["us-east-1"] != 1 || math.Abs(m["eu-central-1"]-1.2) > 1e-6 {
		t.Fatalf("index %v", m)
	}
}

func TestElastiCacheTakesRedisNodesOfARegionOnly(t *testing.T) {
	if _, ok := ElastiCache.Match("Cache Instance", map[string]string{"cacheEngine": "Redis", "locationType": "AWS Outposts", "usagetype": "USE1-Outpost-NodeUsage:cache.m5.large"}); ok {
		t.Fatal("an Outposts node is not a region's price")
	}
	if _, ok := ElastiCache.Match("Cache Instance", map[string]string{"cacheEngine": "Valkey", "locationType": "AWS Region", "usagetype": "NodeUsage:cache.m5.large"}); ok {
		t.Fatal("Valkey is another engine")
	}
	if it, ok := ElastiCache.Match("Cache Instance", map[string]string{"cacheEngine": "Redis", "locationType": "AWS Region", "usagetype": "EUC1-NodeUsage:cache.r5.large"}); !ok || it != "cache.r5.large" {
		t.Fatalf("%q %v", it, ok)
	}
}
