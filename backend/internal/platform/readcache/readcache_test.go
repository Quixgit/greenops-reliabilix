package readcache

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/auth"
)

func call(c *Cache, h http.Handler, tenant, method, uri string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, uri, nil)
	if tenant != "" {
		req = req.WithContext(auth.WithClaims(req.Context(), auth.Claims{Subject: "u", TenantID: tenant}))
	}
	rec := httptest.NewRecorder()
	c.Middleware(h).ServeHTTP(rec, req)
	return rec
}

func TestAnswersAreKeptPerOrganizationAndAddress(t *testing.T) {
	var n atomic.Int32
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"n":` + string(rune('0'+n.Load())) + `}`))
	})
	c := New(time.Minute)
	a1 := call(c, h, "T1", "GET", "/x?from=1")
	a2 := call(c, h, "T1", "GET", "/x?from=1")
	if a1.Body.String() != a2.Body.String() || n.Load() != 1 || a2.Header().Get("X-Cache") != "hit" {
		t.Fatalf("the second ask must be served from memory: %q %q n=%d", a1.Body.String(), a2.Body.String(), n.Load())
	}
	if b := call(c, h, "T2", "GET", "/x?from=1"); n.Load() != 2 || b.Body.String() == a1.Body.String() {
		t.Fatal("another organization must never be served this organization's answer")
	}
	call(c, h, "T1", "GET", "/x?from=2")
	if n.Load() != 3 {
		t.Fatal("another query is another answer")
	}
}

func TestNothingElseIsKept(t *testing.T) {
	var n atomic.Int32
	fail := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { n.Add(1); w.WriteHeader(http.StatusInternalServerError) })
	c := New(time.Minute)
	call(c, fail, "T1", "GET", "/x")
	call(c, fail, "T1", "GET", "/x")
	if n.Load() != 2 {
		t.Fatal("a failure must not be kept")
	}
	ok := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { n.Add(1) })
	n.Store(0)
	call(c, ok, "T1", "POST", "/y")
	call(c, ok, "T1", "POST", "/y")
	call(c, ok, "", "GET", "/z") // no verified organization: never kept
	call(c, ok, "", "GET", "/z")
	if n.Load() != 4 {
		t.Fatalf("a write or an anonymous call must not be kept (ran %d)", n.Load())
	}
	if h := http.Handler(ok); New(0).Middleware(h) == nil {
		t.Fatal("a disabled cache passes through")
	}
}

func TestEntriesExpire(t *testing.T) {
	var n atomic.Int32
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { n.Add(1) })
	c := New(10 * time.Second)
	now := time.Now()
	c.now = func() time.Time { return now }
	call(c, h, "T1", "GET", "/x")
	now = now.Add(11 * time.Second)
	call(c, h, "T1", "GET", "/x")
	if n.Load() != 2 {
		t.Fatal("an answer older than the ttl must be asked again")
	}
}

func TestConcurrentIdenticalRequestsShareOneRun(t *testing.T) {
	var n atomic.Int32
	release := make(chan struct{})
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { n.Add(1); <-release; _, _ = w.Write([]byte("ok")) })
	c := New(time.Minute)
	var wg sync.WaitGroup
	results := make([]string, 8)
	for i := range results {
		wg.Add(1)
		go func() { defer wg.Done(); results[i] = call(c, h, "T1", "GET", "/heavy").Body.String() }()
	}
	time.Sleep(100 * time.Millisecond)
	close(release)
	wg.Wait()
	if n.Load() != 1 {
		t.Fatalf("eight identical requests at once must run once, ran %d", n.Load())
	}
	for _, r := range results {
		if r != "ok" {
			t.Fatalf("every waiter gets the answer: %q", r)
		}
	}
}
