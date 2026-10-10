package eia

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

// A real shape of the answer (strings for values, negative solar at night) and hand-computed intensities.
func TestHourlyComputesTheMixIntensityAndDropsWhatIsNotGeneration(t *testing.T) {
	var gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		fmt.Fprint(w, `{"response":{"total":"13","data":[
		 {"period":"2026-10-09T03","respondent":"CISO","fueltype":"NG","value":"1000","value-units":"megawatthours"},
		 {"period":"2026-10-09T03","respondent":"CISO","fueltype":"NUC","value":"1000","value-units":"megawatthours"},
		 {"period":"2026-10-09T03","respondent":"CISO","fueltype":"SUN","value":"-67","value-units":"megawatthours"},
		 {"period":"2026-10-09T03","respondent":"CISO","fueltype":"BAT","value":"500","value-units":"megawatthours"},
		 {"period":"2026-10-09T04","respondent":"CISO","fueltype":"COL","value":"100","value-units":"megawatthours"},
		 {"period":"2026-10-09T04","respondent":"CISO","fueltype":"WND","value":"300","value-units":"megawatthours"},
		 {"period":"2026-10-09T05","respondent":"CISO","fueltype":"ZZZ","value":"900","value-units":"megawatthours"},
		 {"period":"2026-10-09T05","respondent":"CISO","fueltype":"NG","value":"100","value-units":"megawatthours"}]}}`)
	}))
	defer srv.Close()
	c := New("k")
	c.BaseURL = srv.URL
	from := time.Date(2026, 10, 9, 3, 0, 0, 0, time.UTC)
	got, err := c.Hourly(context.Background(), "CISO", from, from.Add(3*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	// 03h: (1000*490 + 1000*12) / 2000 = 251; negative solar and battery are not generation
	// 04h: (100*820 + 300*11) / 400 = 213.25
	// 05h: 90% of the generation has no known factor: the hour is not used
	if len(got) != 2 || math.Abs(got[0].G-251) > 1e-9 || math.Abs(got[1].G-213.25) > 1e-9 {
		t.Fatalf("%+v", got)
	}
	for _, want := range []string{"facets%5Brespondent%5D%5B%5D=CISO", "frequency=hourly", "start=2026-10-09T03", "end=2026-10-09T05", "api_key=k"} {
		if !strings.Contains(gotQuery, want) {
			t.Errorf("request lacks %s: %s", want, gotQuery)
		}
	}
	// the key must not leak through a transport error
	c.BaseURL = "http://127.0.0.1:1"
	if _, err := c.Hourly(context.Background(), "CISO", from, from.Add(time.Hour)); err == nil || strings.Contains(err.Error(), "api_key") {
		t.Errorf("error must not contain the key: %v", err)
	}
}

func TestRespondentsAndFactorsArePlausible(t *testing.T) {
	if Respondents["US-CAL-CISO"] != "CISO" || Respondents["US-MIDA-PJM"] != "PJM" {
		t.Error("zone mapping")
	}
	if !(Factors["COL"] > Factors["NG"] && Factors["NG"] > Factors["SUN"] && Factors["WND"] < 50 && Factors["NUC"] < 50) {
		t.Errorf("factor order is wrong: %v", Factors)
	}
}
