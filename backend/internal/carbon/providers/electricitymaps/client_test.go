package electricitymaps

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestGetIntensity(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("auth-token") != "k" || r.URL.Query().Get("zone") != "DE" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		_, _ = w.Write([]byte(`{"carbonIntensity":312.5,"datetime":"2026-10-05T12:00:00Z"}`))
	}))
	defer s.Close()
	c := New("k", nil)
	c.BaseURL, c.HTTP = s.URL, s.Client()
	got, err := c.GetIntensity(context.Background(), "eu-central-1")
	if err != nil || got.GPerKWh != 312.5 {
		t.Fatalf("got %+v, %v", got, err)
	}
	if _, err := c.GetIntensity(context.Background(), "mars-1"); err == nil {
		t.Error("unknown region accepted")
	}
}

func TestParseOverrides(t *testing.T) {
	m, err := ParseOverrides(" eu-central-1=DE , ap-southeast-2=AU-NSW ,, ")
	if err != nil || m["eu-central-1"] != "DE" || m["ap-southeast-2"] != "AU-NSW" || len(m) != 2 {
		t.Fatalf("%v %v", m, err)
	}
	for _, bad := range []string{"eu-central-1", "eu-central-1=de", "EU=DE", "eu-central-1=DE/../x", "eu-central-1=DE?x=1", "eu-central-1=" + strings.Repeat("A", 40), "a b=DE"} {
		if _, err := ParseOverrides(bad); err == nil {
			t.Errorf("%q accepted", bad)
		}
	}
	if m, err := ParseOverrides(""); err != nil || len(m) != 0 {
		t.Errorf("empty spec: %v %v", m, err)
	}
}

func TestOverridesAndZones(t *testing.T) {
	c := New("k", map[string]string{"eu-central-1": "DE-X", "xx-test-1": "TT"})
	if c.ZoneMap()["eu-central-1"] != "DE-X" || c.ZoneMap()["us-east-1"] != "US-MIDA-PJM" || c.ZoneMap()["xx-test-1"] != "TT" {
		t.Errorf("zone map: %v", c.ZoneMap())
	}
	if len(c.Regions()) < 25 {
		t.Errorf("only %d regions mapped", len(c.Regions()))
	}
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"DE":{"zoneName":"Germany"},"SE":{"zoneName":"Sweden"}}`))
	}))
	defer s.Close()
	c.BaseURL, c.HTTP = s.URL, s.Client()
	z, err := c.Zones(context.Background())
	if err != nil || len(z) != 2 {
		t.Fatalf("zones: %v %v", z, err)
	}
	if _, ok := z["DE"]; !ok {
		t.Error("DE missing")
	}
}

func TestRedirectsAreNotFollowed(t *testing.T) { // the auth-token header must never follow a redirect to another host
	hit := false
	other := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { hit = true }))
	defer other.Close()
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, other.URL, http.StatusFound) }))
	defer s.Close()
	c := New("secret", nil)
	c.BaseURL = s.URL
	if _, err := c.GetIntensity(context.Background(), "eu-central-1"); err == nil || hit {
		t.Errorf("redirect followed (hit=%v, err=%v)", hit, err)
	}
}

func TestResponsesAreCachedAndExpire(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		_, _ = w.Write([]byte(`{"carbonIntensity":100,"datetime":"2026-01-01T00:00:00Z"}`))
	}))
	defer srv.Close()
	c := New("k", nil)
	c.BaseURL = srv.URL
	now := time.Now()
	c.now = func() time.Time { return now }

	for i := 0; i < 3; i++ {
		if _, err := c.GetIntensity(context.Background(), "eu-central-1"); err != nil {
			t.Fatal(err)
		}
	}
	if calls != 1 {
		t.Fatalf("calls = %d, want 1 (cached)", calls)
	}
	now = now.Add(intensityTTL + time.Second)
	if _, err := c.GetIntensity(context.Background(), "eu-central-1"); err != nil || calls != 2 {
		t.Fatalf("calls = %d err = %v, want a refetch after expiry", calls, err)
	}
}

func TestFailuresAreNotCached(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer srv.Close()
	c := New("k", nil)
	c.BaseURL = srv.URL
	for i := 0; i < 2; i++ {
		if _, err := c.GetIntensity(context.Background(), "eu-central-1"); err == nil {
			t.Fatal("want an error")
		}
	}
	if calls != 2 {
		t.Fatalf("calls = %d, want 2 (errors must be retried)", calls)
	}
}
