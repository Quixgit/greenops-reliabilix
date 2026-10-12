package azureprice

import (
	"context"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func row(region, sku, product, skuName string, price float64) string {
	return fmt.Sprintf(`{"retailPrice":%v,"armRegionName":%q,"armSkuName":%q,"skuName":%q,"productName":%q,"meterName":"x","type":"Consumption"}`, price, region, sku, skuName, product)
}

func TestIndexIsTheGeometricRatioOfLinuxOnDemandPrices(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		region := "eastus"
		if strings.Contains(r.URL.RawQuery, "westeurope") {
			region = "westeurope"
		}
		k := 1.0
		if region == "westeurope" {
			k = 1.2
		}
		var items []string
		for _, s := range Basket[:5] {
			items = append(items, row(region, s, "Virtual Machines Dsv5 Series", s, 0.1*k))
			items = append(items, row(region, s, "Virtual Machines Dsv5 Series Windows", s, 0.5*k)) // never the Linux price
			items = append(items, row(region, s, "Virtual Machines Dsv5 Series", s+" Spot", 0.01*k))
		}
		_, _ = w.Write([]byte(`{"Items":[` + strings.Join(items, ",") + `]}`))
	}))
	defer srv.Close()
	c := New()
	c.BaseURL = srv.URL
	idx, errs, err := c.BuildIndex(context.Background(), []string{"westeurope"})
	if err != nil || len(errs) != 0 {
		t.Fatalf("%v %v", err, errs)
	}
	m := map[string]float64{}
	for _, i := range idx {
		m[i.Region] = i.Idx
	}
	if m["eastus"] != 1 || math.Abs(m["westeurope"]-1.2) > 1e-6 {
		t.Fatalf("index %v", m)
	}
}

func TestTooFewSizesIsNotAnIndex(t *testing.T) {
	if _, n, ok := Ratio(map[string]float64{"Standard_D2s_v5": 1}, map[string]float64{"Standard_D2s_v5": 1}); ok || n != 1 {
		t.Fatalf("n=%d ok=%v", n, ok)
	}
}

func TestPostgreSQLIsPricedPerVCore(t *testing.T) {
	prod := "Azure Database for PostgreSQL Flexible Server General Purpose Ddsv5 Series Compute"
	prod2 := "Azure Database for PostgreSQL Flexible Server Memory Optimized Edsv5 Series Compute"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		k := 1.0
		region := "eastus"
		if strings.Contains(r.URL.RawQuery, "canadacentral") {
			k, region = 1.1, "canadacentral"
		}
		items := []string{
			row(region, "", prod, "2 vCore", 0.178*k),                                                // 2 vCore: 0.089 each
			row(region, "", prod, "32 vCore", 2.848*k),                                               // 32 vCore: 0.089 each, the same price per core
			row(region, "", prod2, "vCore", 0.125*k),                                                 // one vCore
			row(region, "", "Azure Database for PostgreSQL Flex Server Storage", "Storage", 0.115*k), // storage is not compute
		}
		_, _ = w.Write([]byte(`{"Items":[` + strings.Join(items, ",") + `]}`))
	}))
	defer srv.Close()
	c := New()
	c.BaseURL = srv.URL
	p, err := c.FamilyPrices(context.Background(), PostgreSQL, "eastus")
	if err != nil || len(p) != 2 || math.Abs(p[prod]-0.089) > 1e-9 || math.Abs(p[prod2]-0.125) > 1e-9 {
		t.Fatalf("%v %v", p, err)
	}
	idx, errs, err := c.BuildFamilyIndex(context.Background(), PostgreSQL, []string{"canadacentral"})
	if err != nil || len(errs) != 0 {
		t.Fatalf("%v %v", err, errs)
	}
	got := map[string]float64{}
	for _, i := range idx {
		got[i.Region] = i.Idx
	}
	if math.Abs(got["canadacentral"]-1.1) > 1e-6 {
		t.Fatalf("index %v", got)
	}
}

func TestRedisNeedsEnoughOfTheBasket(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		items := []string{
			row("eastus", "", "Azure Redis Cache Standard", "C1", 0.138), row("eastus", "", "Azure Redis Cache Standard", "C3", 0.225),
			row("eastus", "", "Azure Redis Cache Basic", "C1", 0.055), row("eastus", "", "Azure Redis Cache Enterprise", "E20", 0.96),
		}
		_, _ = w.Write([]byte(`{"Items":[` + strings.Join(items, ",") + `]}`))
	}))
	defer srv.Close()
	c := New()
	c.BaseURL = srv.URL
	p, err := c.FamilyPrices(context.Background(), Redis, "eastus")
	if err != nil || len(p) != 3 || p["Azure Redis Cache Standard/C1"] != 0.138 {
		t.Fatalf("%v %v", p, err) // Enterprise is another product: left out
	}
}

func TestSlug(t *testing.T) {
	for in, want := range map[string]string{"Azure App Service": "azure_app_service", "Functions": "functions", " Foundry  Models ": "foundry_models", "SQL/DB (v2)": "sql_db_v2"} {
		if got := Slug(in); got != want {
			t.Errorf("Slug(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestGenericRatioComparesEveryMeterBothRegionsHave(t *testing.T) {
	fam := GenericFamily("Azure Container Apps")
	base := map[string]float64{"a": 1, "b": 2, "c": 4, "d": 8, "e": 3, "only-base": 5}
	region := map[string]float64{"a": 1.1, "b": 2.2, "c": 4.4, "d": 8.8, "e": 3.3, "only-region": 9}
	r, n, ok := familyRatio(fam, region, base)
	if !ok || n != 5 || r < 1.0999 || r > 1.1001 {
		t.Fatalf("ratio %v over %d meters, ok=%v", r, n, ok)
	}
	// too few meters in common: no figure at all rather than a guess from two prices
	if _, n, ok := familyRatio(fam, map[string]float64{"a": 2, "b": 4}, base); ok || n != 2 {
		t.Fatalf("two meters must not make an index (n=%d ok=%v)", n, ok)
	}
}

func TestGenericKeySkipsSpot(t *testing.T) {
	fam := GenericFamily("Azure App Service")
	if _, _, ok := fam.Key(item{Product: "p", SKUName: "S1 Spot", Meter: "m", Price: 1}); ok {
		t.Error("spot prices are not comparable")
	}
	if k, _, ok := fam.Key(item{Product: "p", SKUName: "S1", Meter: "m", Price: 1}); !ok || k != "p|S1|m" {
		t.Errorf("key %q ok=%v", k, ok)
	}
}
