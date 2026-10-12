package watttime

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// server stands in for WattTime: /login needs Basic auth, the data endpoints need the bearer token it issued.
func server(t *testing.T, logins *int32, data http.HandlerFunc) *Client {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/login", func(w http.ResponseWriter, r *http.Request) {
		u, p, ok := r.BasicAuth()
		if !ok || u != "user" || p != "pw" {
			http.Error(w, "no", http.StatusUnauthorized)
			return
		}
		n := atomic.AddInt32(logins, 1)
		_, _ = w.Write([]byte(`{"token":"tok` + string(rune('0'+n)) + `"}`))
	})
	mux.HandleFunc("/v3/", func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.Header.Get("Authorization"), "Bearer tok") {
			http.Error(w, "no", http.StatusUnauthorized)
			return
		}
		data(w, r)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	c := New("user", "pw")
	c.BaseURL = srv.URL
	return c
}

func TestHistoricalConvertsPoundsPerMWhToGramsPerKWh(t *testing.T) {
	var logins int32
	c := server(t, &logins, func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if q.Get("region") != "CAISO_NORTH" || q.Get("signal_type") != "co2_moer" {
			t.Errorf("query: %v", q)
		}
		_, _ = w.Write([]byte(`{"data":[{"point_time":"2026-10-01T00:00:00+00:00","value":1000},{"point_time":"2026-10-01T00:05:00+00:00","value":null},
		  {"point_time":"2026-10-01T00:10:00+00:00","value":99999},{"point_time":"bad","value":5}],"meta":{"units":"lbs_co2_per_mwh"}}`))
	})
	from := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	got, err := c.Historical(context.Background(), "CAISO_NORTH", from, from.Add(time.Hour))
	if err != nil || len(got) != 1 {
		t.Fatalf("one plausible point expected (null, absurd and unreadable ones skipped): %v %v", got, err)
	}
	if d := got[0].G - 453.59237; d > 1e-6 || d < -1e-6 {
		t.Errorf("1000 lb/MWh is 453.59 g/kWh, got %v", got[0].G)
	}
}

func TestUnknownOrMissingUnitIsRefusedNotGuessed(t *testing.T) {
	var logins int32
	c := server(t, &logins, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"data":[{"point_time":"2026-10-01T00:00:00+00:00","value":10}],"meta":{}}`))
	})
	from := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	if _, err := c.Historical(context.Background(), "CAISO_NORTH", from, from.Add(time.Hour)); err == nil || !strings.Contains(err.Error(), "unknown unit") {
		t.Fatalf("a response without a unit must be refused: %v", err)
	}
}

func TestTokenIsReusedAndRenewedOnceWhenRejected(t *testing.T) {
	var logins, calls int32
	c := server(t, &logins, func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt32(&calls, 1) == 2 && r.Header.Get("Authorization") == "Bearer tok1" {
			http.Error(w, "expired", http.StatusUnauthorized) // the token WattTime issued has expired
			return
		}
		_, _ = w.Write([]byte(`{"region":"CAISO_NORTH","region_full_name":"x"}`))
	})
	for i := 0; i < 3; i++ {
		if _, err := c.Region(context.Background(), 37.77, -122.42); err != nil {
			t.Fatal(err)
		}
	}
	if logins != 2 {
		t.Errorf("one login, then one more after the rejection, got %d", logins)
	}
}

func TestBadCredentialsAndNoAccessAreDistinct(t *testing.T) {
	var logins int32
	c := server(t, &logins, func(w http.ResponseWriter, _ *http.Request) { http.Error(w, "paid", http.StatusForbidden) })
	if err := c.Check(context.Background()); !errors.Is(err, ErrNoAccess) {
		t.Errorf("403 on data is a plan limit: %v", err)
	}
	c.Password = "wrong"
	c.token = ""
	err := c.Check(context.Background())
	if !errors.Is(err, ErrCredentials) || strings.Contains(err.Error(), "wrong") {
		t.Errorf("a rejected login names no secret: %v", err)
	}
	if err := New("", "").Check(context.Background()); !errors.Is(err, ErrNoCredentials) {
		t.Errorf("no credentials: %v", err)
	}
}

func TestInputsAreValidatedBeforeAnyRequest(t *testing.T) {
	c := New("u", "p")
	c.BaseURL = "http://127.0.0.1:1" // would fail to connect: validation must come first
	ctx := context.Background()
	if _, err := c.Historical(ctx, "../etc", time.Now().Add(-time.Hour), time.Now()); err == nil || !strings.Contains(err.Error(), "invalid region") {
		t.Errorf("region: %v", err)
	}
	if _, err := c.Forecast(ctx, "CAISO_NORTH", 200); err == nil || !strings.Contains(err.Error(), "horizon") {
		t.Errorf("horizon: %v", err)
	}
	if _, err := c.Region(ctx, 99, 0); err == nil {
		t.Error("latitude out of range")
	}
}

func TestForecastReadsThePlanningHorizon(t *testing.T) {
	var logins int32
	soon := time.Now().UTC().Add(time.Hour).Format(time.RFC3339)
	c := server(t, &logins, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v3/forecast" || r.URL.Query().Get("horizon_hours") != "24" {
			t.Errorf("request: %s?%s", r.URL.Path, r.URL.RawQuery)
		}
		_, _ = w.Write([]byte(`{"data":[{"point_time":"` + soon + `","value":500}],"meta":{"units":"lbs_co2_per_mwh"}}`))
	})
	got, err := c.Forecast(context.Background(), "CAISO_NORTH", 24)
	if err != nil || len(got) != 1 || got[0].G < 226 || got[0].G > 228 {
		t.Fatalf("%v %v", got, err)
	}
}
