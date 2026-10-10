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

func listing(loc string, prices map[string]string) string {
	var rows []string
	for it, p := range prices {
		rows = append(rows, fmt.Sprintf(`%q:{"price":%q,"Instance Type":%q,"Operating System":"Linux","Pre Installed S/W":"NA"}`, strings.ReplaceAll(it, ".", " ")+" x", p, it))
	}
	return fmt.Sprintf(`{"manifest":{},"regions":{%q:{%s}},"sets":{}}`, loc, strings.Join(rows, ","))
}

func TestBuildIndexIsAGeometricMeanOfBasketRatiosAndSkipsFailures(t *testing.T) {
	base := map[string]string{"m5.large": "0.100", "m5.xlarge": "0.200", "c5.large": "0.085", "c5.xlarge": "0.170", "r5.large": "0.126"}
	fra := map[string]string{"m5.large": "0.115", "m5.xlarge": "0.230", "c5.large": "0.0978", "c5.xlarge": "0.1955", "r5.large": "0.1449"} // every one is 15% dearer
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.RawPath+r.URL.Path, "N. Virginia"):
			fmt.Fprint(w, listing("US East (N. Virginia)", base))
		case strings.Contains(r.URL.Path, "Frankfurt"):
			fmt.Fprint(w, listing("EU (Frankfurt)", fra))
		case strings.Contains(r.URL.Path, "Tokyo"):
			fmt.Fprint(w, listing("Asia Pacific (Tokyo)", map[string]string{"m5.large": "0.12"})) // too few in common
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()
	c := New()
	c.BaseURL = srv.URL
	idx, errs, err := c.BuildIndex(context.Background(), []string{"eu-central-1", "ap-northeast-1", "eu-west-3"})
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]float64{}
	for _, i := range idx {
		got[i.Region] = i.Idx
	}
	if got["us-east-1"] != 1 || math.Abs(got["eu-central-1"]-1.15) > 1e-3 {
		t.Fatalf("index %+v", got)
	}
	if _, ok := got["ap-northeast-1"]; ok || errs["ap-northeast-1"] == nil || errs["eu-west-3"] == nil {
		t.Fatalf("a region with too little overlap or a failing page must be skipped and reported: %+v %v", got, errs)
	}
}

func TestPlainVariantWinsAndNonLinuxIsIgnored(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"regions":{"X":{
		 "a":{"price":"0.30","Instance Type":"m5.large","Operating System":"Linux","Pre Installed S/W":"NA"},
		 "b":{"price":"0.10","Instance Type":"m5.large","Operating System":"Linux","Pre Installed S/W":"NA"},
		 "c":{"price":"0.01","Instance Type":"m5.large","Operating System":"Windows","Pre Installed S/W":"NA"},
		 "d":{"price":"0.02","Instance Type":"m5.large","Operating System":"Linux","Pre Installed S/W":"SQL Std"}}}}`)
	}))
	defer srv.Close()
	c := New()
	c.BaseURL = srv.URL
	p, err := c.Prices(context.Background(), "us-east-1")
	if err != nil || p["m5.large"] != 0.10 {
		t.Fatalf("%v %v", p, err)
	}
}

func TestEveryLocationIsKnownForTheRegionsWeMapOnTheMap(t *testing.T) {
	if len(Locations) < 20 {
		t.Fatal("location table shrank")
	}
}
