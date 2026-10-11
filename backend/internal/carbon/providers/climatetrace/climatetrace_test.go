package climatetrace

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestPowerTonnes(t *testing.T) {
	var q map[string][]string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q = r.URL.Query()
		_, _ = w.Write([]byte(`[{"country":"BRA","emissions":{"co2e_100yr":46682429.7}},{"country":"DEU","emissions":{"co2e_100yr":199294745.4}},{"country":"XXX","emissions":{"co2e_100yr":0}}]`))
	}))
	defer srv.Close()
	c := New()
	c.BaseURL = srv.URL
	got, err := c.PowerTonnes(context.Background(), []string{"BRA", "DEU", "XXX"}, 2023)
	if err != nil || len(got) != 2 || got["BRA"] != 46682429.7 || got["DEU"] != 199294745.4 {
		t.Fatalf("%v %v", got, err)
	}
	if q["since"][0] != "2023" || q["to"][0] != "2023" || q["sectors"][0] != "power" || q["countries"][0] != "BRA,DEU,XXX" {
		t.Fatalf("query: %v", q)
	}
}

func TestBadStatusIsAnError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusBadGateway) }))
	defer srv.Close()
	c := New()
	c.BaseURL = srv.URL
	if _, err := c.PowerTonnes(context.Background(), []string{"DEU"}, 2023); err == nil {
		t.Fatal("expected an error")
	}
}
