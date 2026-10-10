package energycharts

import (
	"context"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// Four quarter-hours of one hour, plus storage, demand and a null: hand-computed intensity.
func TestHourlyAveragesQuarterHoursAndLeavesOutWhatIsNotGeneration(t *testing.T) {
	var gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		t0 := time.Date(2026, 10, 9, 3, 0, 0, 0, time.UTC).Unix()
		fmt.Fprintf(w, `{"unix_seconds":[%d,%d,%d,%d,%d],"production_types":[
		 {"name":"Fossil gas","data":[1000,1000,1000,1000,500]},
		 {"name":"Nuclear","data":[1000,1000,1000,1000,500]},
		 {"name":"Hydro pumped storage consumption","data":[-900,-900,-900,-900,-900]},
		 {"name":"Hydro pumped storage","data":[900,900,900,900,900]},
		 {"name":"Load","data":[50000,50000,50000,50000,50000]},
		 {"name":"Cross border electricity trading","data":[null,null,null,null,null]},
		 {"name":"Renewable share of load","data":[70,70,70,70,70]}]}`, t0, t0+900, t0+1800, t0+2700, t0+3600)
	}))
	defer srv.Close()
	c := New()
	c.BaseURL = srv.URL
	from := time.Date(2026, 10, 9, 3, 0, 0, 0, time.UTC)
	got, err := c.Hourly(context.Background(), "de", from, from.Add(2*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	// both hours: gas and nuclear in equal parts = (490 + 12) / 2 = 251; storage, load, shares and trading are not generation
	if len(got) != 2 || math.Abs(got[0].G-251) > 1e-9 || math.Abs(got[1].G-251) > 1e-9 {
		t.Fatalf("got %+v", got)
	}
	if !strings.Contains(gotQuery, "country=de") || !strings.Contains(gotQuery, "start=2026-10-09") {
		t.Fatalf("query %q", gotQuery)
	}
}

func TestAnUnknownTypeCountsAsOtherAndTooMuchOfItDropsTheHour(t *testing.T) {
	t0 := time.Date(2026, 10, 9, 3, 0, 0, 0, time.UTC).Unix()
	v := func(x float64) *float64 { return &x }
	got := mix(body{UnixSeconds: []int64{t0}, ProductionTypes: []series{{Name: "Fossil gas", Data: []*float64{v(100)}}, {Name: "Mystery", Data: []*float64{v(900)}}}})
	if len(got) != 0 {
		t.Fatalf("90%% unknown must not be used: %+v", got)
	}
}

func TestFailuresDoNotLeakAndAreSurfaced(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusTooManyRequests) }))
	defer srv.Close()
	c := New()
	c.BaseURL = srv.URL
	failed := false
	c.Done = func(_ context.Context, f bool) { failed = f }
	if _, err := c.Hourly(context.Background(), "de", time.Now().Add(-48*time.Hour), time.Now()); err == nil || !failed {
		t.Fatalf("err=%v failed=%v", err, failed)
	}
}
