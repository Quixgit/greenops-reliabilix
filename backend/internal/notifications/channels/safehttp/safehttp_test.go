package safehttp

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"testing"

	"github.com/quixgit/greenops-reliabilix/backend/internal/notifications/domain"
)

func TestParseURL(t *testing.T) {
	good := []string{"https://example.com/hook", "https://hooks.example.com:8443/a?b=c", "https://8.8.8.8/x"}
	for _, u := range good {
		if _, err := ParseURL(u, Options{}); err != nil {
			t.Errorf("ParseURL(%q) = %v", u, err)
		}
	}
	bad := []string{
		"", "example.com/hook", "http://example.com/hook", "ftp://example.com", "https://", "https://user:pw@example.com/",
		"https://example.com/#frag", "https://example.com/a b", "https://localhost/x", "https://api.localhost/x",
		"https://db.internal/x", "https://printer.local/x", "https://127.0.0.1/x", "https://10.0.0.5/x",
		"https://169.254.169.254/latest/meta-data", "https://[::1]/x", "https://192.168.1.1/x", "https://100.64.0.1/x",
		"https://" + strings.Repeat("a", 2100) + ".com",
	}
	for _, u := range bad {
		if _, err := ParseURL(u, Options{}); !errors.Is(err, domain.ErrInvalid) {
			t.Errorf("ParseURL(%q) = %v, want ErrInvalid", u, err)
		}
	}
	if _, err := ParseURL("http://127.0.0.1:8080/x", Options{AllowHTTP: true, AllowPrivate: true}); err != nil {
		t.Errorf("development options must allow a local http receiver: %v", err)
	}
}

func TestIsPublic(t *testing.T) {
	for ip, want := range map[string]bool{
		"8.8.8.8": true, "1.1.1.1": true, "2606:4700:4700::1111": true,
		"127.0.0.1": false, "10.1.2.3": false, "172.16.0.1": false, "192.168.0.1": false, "169.254.169.254": false,
		"0.0.0.0": false, "100.64.0.1": false, "224.0.0.1": false, "::1": false, "fe80::1": false, "fc00::1": false,
		"::ffff:127.0.0.1": false, "::ffff:10.0.0.1": false, "255.255.255.255": false, "2001:db8::1": false,
	} {
		if got := isPublic(netip.MustParseAddr(ip)); got != want {
			t.Errorf("isPublic(%s) = %v, want %v", ip, got, want)
		}
	}
}

// The dialer must refuse a loopback server even though the URL itself looked acceptable (this is what protects
// against a public host name that resolves to a private address).
func TestClientRefusesPrivateAddressesAtDialTime(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Error("the request must never reach a loopback server")
	}))
	defer srv.Close()
	err := PostJSON(context.Background(), NewClient(Options{}), srv.URL, nil, []byte("{}"))
	if err == nil || !domain.IsPermanent(err) {
		t.Fatalf("PostJSON to loopback = %v, want a permanent refusal", err)
	}
	if strings.Contains(err.Error(), srv.URL) || strings.Contains(err.Error(), "127.0.0.1") {
		t.Errorf("error leaks the address: %v", err)
	}
}

func TestPostJSONOutcomes(t *testing.T) {
	var seen *http.Request
	status := http.StatusOK
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = r
		w.WriteHeader(status)
	}))
	defer srv.Close()
	c := NewClient(Options{AllowPrivate: true})

	if err := PostJSON(context.Background(), c, srv.URL, map[string]string{"X-Test": "1"}, []byte(`{"a":1}`)); err != nil {
		t.Fatalf("2xx must succeed: %v", err)
	}
	if seen.Method != "POST" || seen.Header.Get("Content-Type") != "application/json" || seen.Header.Get("X-Test") != "1" {
		t.Errorf("unexpected request: %v %v", seen.Method, seen.Header)
	}
	for code, permanent := range map[int]bool{400: true, 401: true, 403: true, 404: true, 410: true, 302: true, 429: false, 408: false, 500: false, 503: false} {
		status = code
		err := PostJSON(context.Background(), c, srv.URL, nil, []byte("{}"))
		if err == nil || domain.IsPermanent(err) != permanent {
			t.Errorf("HTTP %d: err=%v permanent=%v, want permanent=%v", code, err, domain.IsPermanent(err), permanent)
		}
		if err != nil && strings.Contains(err.Error(), srv.URL) {
			t.Errorf("HTTP %d: error leaks the URL", code)
		}
	}
}

func TestRedirectsAreNotFollowed(t *testing.T) {
	hit := false
	target := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { hit = true }))
	defer target.Close()
	redir := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL, http.StatusFound)
	}))
	defer redir.Close()
	err := PostJSON(context.Background(), NewClient(Options{AllowPrivate: true}), redir.URL, nil, []byte("{}"))
	if err == nil || !domain.IsPermanent(err) || hit {
		t.Errorf("redirect must be a permanent failure and not be followed (err=%v hit=%v)", err, hit)
	}
}

func TestTransportErrorsAreSafe(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	url := srv.URL + "/secret-token"
	srv.Close() // connection refused
	err := PostJSON(context.Background(), NewClient(Options{AllowPrivate: true}), url, nil, []byte("{}"))
	if err == nil || domain.IsPermanent(err) {
		t.Fatalf("a refused connection is transient, got %v", err)
	}
	if strings.Contains(err.Error(), "secret-token") || strings.Contains(err.Error(), "127.0.0.1") {
		t.Errorf("error leaks the URL: %v", err)
	}
}
