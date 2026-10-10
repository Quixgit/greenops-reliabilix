package ember

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestMonthlyParsesDropsImplausibleAndHidesTheKey(t *testing.T) {
	var gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		_, _ = w.Write([]byte(`{"data":[{"date":"2025-01","emissions_intensity_gco2_per_kwh":351.2},{"date":"2025-02","emissions_intensity_gco2_per_kwh":0},{"date":"2025-03","emissions_intensity_gco2_per_kwh":288.4}]}`))
	}))
	defer srv.Close()
	c := New("secret-key")
	c.BaseURL = srv.URL
	got, err := c.Monthly(context.Background(), "DEU", time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC), time.Date(2025, 3, 1, 0, 0, 0, 0, time.UTC))
	if err != nil || len(got) != 2 || got[0].G != 351.2 || got[0].Month.Month() != 1 || got[1].Month.Month() != 3 {
		t.Fatalf("%+v %v", got, err)
	}
	for _, want := range []string{"entity_code=DEU", "start_date=2025-01", "end_date=2025-03", "api_key=secret-key"} {
		if !strings.Contains(gotQuery, want) {
			t.Errorf("request lacks %s: %s", want, gotQuery)
		}
	}
	// an unreachable server must not leak the key through the error
	c.BaseURL = "http://127.0.0.1:1"
	if _, err := c.Monthly(context.Background(), "DEU", time.Now().AddDate(0, -2, 0), time.Now()); err == nil || strings.Contains(err.Error(), "secret-key") {
		t.Errorf("error must not contain the key: %v", err)
	}
}

func TestCountryMappingAndCap(t *testing.T) {
	for zone, want := range map[string]string{"DE": "DEU", "US-CAL-CISO": "USA", "GB": "GBR", "JP-TK": "JPN"} {
		if got, ok := CountryOf(zone); !ok || got != want {
			t.Errorf("CountryOf(%s) = %s %v", zone, got, ok)
		}
	}
	if _, ok := CountryOf("XX"); ok {
		t.Error("an unknown country must not be guessed")
	}
	denied := New("k")
	denied.Allow = func(context.Context) error { return http.ErrAbortHandler }
	if _, err := denied.Monthly(context.Background(), "DEU", time.Now(), time.Now()); err == nil {
		t.Error("a request over the cap must not be made")
	}
}
