package electricitymaps

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
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
	c.now = func() time.Time { return time.Date(2026, 10, 5, 12, 20, 0, 0, time.UTC) } // the reading is 20 minutes old
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
	now := time.Date(2026, 1, 1, 0, 10, 0, 0, time.UTC)
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
	if _, err := c.GetIntensity(context.Background(), "eu-central-1"); err != nil || calls != 2 { //nolint:staticcheck
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
	c.sleep = func(time.Duration) {}
	for i := 0; i < 2; i++ {
		if _, err := c.GetIntensity(context.Background(), "eu-central-1"); err == nil {
			t.Fatal("want an error")
		}
	}
	if calls != 2*maxAttempts {
		t.Fatalf("calls = %d, want %d: each call retries a bad gateway %d times and a failure is never cached", calls, 2*maxAttempts, maxAttempts)
	}
}

func TestGoogleCloudRegionsAreMapped(t *testing.T) {
	c := New("k", nil)
	for region, zone := range map[string]string{"europe-west3": "DE", "europe-north1": "FI", "us-west1": "US-NW-PACW", "asia-northeast1": "JP-TK"} {
		if got := c.ZoneMap()[region]; got != zone {
			t.Errorf("%s -> %q, want %q", region, got, zone)
		}
	}
	if c.ZoneMap()["eu-central-1"] != "DE" {
		t.Error("AWS regions must keep working next to Google Cloud regions")
	}
}

func okBody(g float64, at time.Time) []byte {
	return []byte(fmt.Sprintf(`{"carbonIntensity":%v,"datetime":%q}`, g, at.Format(time.RFC3339)))
}

func TestTransientErrorsAreRetriedButAuthErrorsAreNot(t *testing.T) {
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		if calls < 3 {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		_, _ = w.Write(okBody(200, time.Now()))
	}))
	c := New("k", nil)
	c.BaseURL, c.sleep = srv.URL, func(time.Duration) {}
	if g, err := c.GetIntensity(context.Background(), "eu-central-1"); err != nil || g.GPerKWh != 200 || calls != 3 {
		t.Fatalf("429 twice then OK: %+v err=%v calls=%d", g, err, calls)
	}
	srv.Close()

	calls = 0
	auth := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { calls++; w.WriteHeader(http.StatusUnauthorized) }))
	defer auth.Close()
	c2 := New("bad", nil)
	c2.BaseURL, c2.sleep = auth.URL, func(time.Duration) {}
	if _, err := c2.GetIntensity(context.Background(), "eu-central-1"); err == nil || calls != 1 {
		t.Errorf("a 401 is final (calls=%d, err=%v): retrying only burns quota", calls, err)
	}
}

func TestErrorsNeverContainTheURLOrKey(t *testing.T) {
	c := New("super-secret-key", nil)
	c.BaseURL, c.sleep = "http://127.0.0.1:1/?token=abc", func(time.Duration) {}
	_, err := c.GetIntensity(context.Background(), "eu-central-1")
	if err == nil || strings.Contains(err.Error(), "127.0.0.1") || strings.Contains(err.Error(), "super-secret") || strings.Contains(err.Error(), "token=") {
		t.Errorf("error leaks request details: %v", err)
	}
}

func TestImplausibleReadingsAreRefused(t *testing.T) {
	now := time.Date(2026, 10, 5, 12, 0, 0, 0, time.UTC)
	for name, body := range map[string][]byte{
		"zero (an unpopulated response)": okBody(0, now),
		"negative":                       okBody(-5, now),
		"absurdly high":                  okBody(99999, now),
		"too old to describe now":        okBody(300, now.Add(-7*time.Hour)),
		"no timestamp":                   []byte(`{"carbonIntensity":300}`),
	} {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write(body) }))
		c := New("k", nil)
		c.BaseURL, c.now = srv.URL, func() time.Time { return now }
		if g, err := c.GetIntensity(context.Background(), "eu-central-1"); err == nil {
			t.Errorf("%s: accepted %+v", name, g)
		}
		srv.Close()
	}
}

// When the provider is down, a recent reading keeps the dashboard and jobs working; an old one does not pretend.
func TestStaleReadingServesDuringAnOutageButOnlyBriefly(t *testing.T) {
	down := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if down {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		_, _ = w.Write(okBody(250, time.Date(2026, 10, 5, 12, 0, 0, 0, time.UTC)))
	}))
	defer srv.Close()
	now := time.Date(2026, 10, 5, 12, 5, 0, 0, time.UTC)
	c := New("k", nil)
	c.BaseURL, c.sleep, c.now = srv.URL, func(time.Duration) {}, func() time.Time { return now }
	if _, err := c.GetIntensity(context.Background(), "eu-central-1"); err != nil {
		t.Fatal(err)
	}
	down = true
	now = now.Add(intensityTTL + time.Minute) // expired, provider down
	if g, err := c.GetIntensity(context.Background(), "eu-central-1"); err != nil || g.GPerKWh != 250 {
		t.Errorf("an hour-old reading should serve during an outage: %+v %v", g, err)
	}
	now = now.Add(staleAfterError)
	if _, err := c.GetIntensity(context.Background(), "eu-central-1"); err == nil {
		t.Error("a reading older than the stale limit must not be served")
	}
}

func TestConcurrentCallersShareOneLoad(t *testing.T) {
	var calls atomic.Int32
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		<-release
		_, _ = w.Write(okBody(111, time.Now()))
	}))
	defer srv.Close()
	c := New("k", nil)
	c.BaseURL = srv.URL
	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() { defer wg.Done(); _, _ = c.GetIntensity(context.Background(), "eu-central-1") }()
	}
	time.Sleep(100 * time.Millisecond)
	close(release)
	wg.Wait()
	if n := calls.Load(); n != 1 {
		t.Errorf("provider was called %d times for 8 concurrent callers, want 1", n)
	}
}
