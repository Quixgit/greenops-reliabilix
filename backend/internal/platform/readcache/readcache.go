// Package readcache keeps the answers of the heavy read endpoints for a few seconds and lets concurrent identical requests share one
// execution (ADR-0060). A dashboard question over a year of line items costs seconds of database time and is asked, nearly identically, by
// every person of an organization and every widget refresh; without this, twenty people opening the Overview at once is twenty times the work.
//
// The key is the organization of the verified identity plus the full request address, so one organization never sees another's answer, and it
// is used only after authorization (the middleware sits behind auth.Require). Only successful GET answers are kept, and only small ones.
package readcache

import (
	"bytes"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/sync/singleflight"

	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/auth"
)

const (
	maxEntries = 512
	maxBytes   = 2 << 20
)

type entry struct {
	status  int
	ctype   string
	body    []byte
	expires time.Time
}

// Cache is safe for concurrent use. The zero value is not usable: use New.
type Cache struct {
	ttl time.Duration
	now func() time.Time

	mu      sync.Mutex
	entries map[string]entry
	flight  singleflight.Group
}

// New keeps answers for ttl; a ttl of zero or less disables caching (requests still run as they would without it).
func New(ttl time.Duration) *Cache {
	return &Cache{ttl: ttl, now: time.Now, entries: map[string]entry{}}
}

func (c *Cache) get(key string) (entry, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.entries[key]
	if !ok || !c.now().Before(e.expires) {
		delete(c.entries, key)
		return entry{}, false
	}
	return e, true
}

func (c *Cache) put(key string, e entry) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.entries) >= maxEntries {
		now := c.now()
		for k, v := range c.entries {
			if !now.Before(v.expires) {
				delete(c.entries, k)
			}
		}
		if len(c.entries) >= maxEntries { // all still fresh: drop an arbitrary one rather than grow
			for k := range c.entries {
				delete(c.entries, k)
				break
			}
		}
	}
	c.entries[key] = e
}

// recorder captures what a handler writes.
type recorder struct {
	h      http.Header
	status int
	body   bytes.Buffer
}

func (r *recorder) Header() http.Header         { return r.h }
func (r *recorder) WriteHeader(code int)        { r.status = code }
func (r *recorder) Write(b []byte) (int, error) { return r.body.Write(b) }

// Middleware serves a kept answer, or runs the handler once for all who ask the same thing at the same time.
func (c *Cache) Middleware(next http.Handler) http.Handler {
	if c == nil || c.ttl <= 0 {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cl, ok := auth.FromContext(r.Context())
		if r.Method != http.MethodGet || !ok || cl.TenantID == "" {
			next.ServeHTTP(w, r)
			return
		}
		key := cl.TenantID + "|" + r.URL.RequestURI()
		if e, hit := c.get(key); hit {
			write(w, e, "hit")
			return
		}
		v, _, shared := c.flight.Do(key, func() (any, error) {
			rec := &recorder{h: http.Header{}, status: http.StatusOK}
			next.ServeHTTP(rec, r)
			e := entry{status: rec.status, ctype: rec.h.Get("Content-Type"), body: rec.body.Bytes()}
			if e.status == http.StatusOK && len(e.body) <= maxBytes {
				e.expires = c.now().Add(c.ttl)
				c.put(key, e)
			}
			return e, nil
		})
		e := v.(entry)
		state := "miss"
		if shared {
			state = "shared"
		}
		write(w, e, state)
	})
}

func write(w http.ResponseWriter, e entry, state string) {
	if e.ctype != "" {
		w.Header().Set("Content-Type", e.ctype)
	}
	w.Header().Set("X-Cache", state)
	w.WriteHeader(e.status)
	_, _ = w.Write(e.body)
}

var def atomic.Pointer[Cache]

// SetDefault chooses the cache the routes use (nil, or a cache with a ttl of zero, turns it off). It is read on every request, so it may be
// set after the routes were registered.
func SetDefault(c *Cache) { def.Store(c) }

// Middleware is the middleware the routes name: it uses the default cache, and does nothing until one is set.
func Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if c := def.Load(); c != nil {
			c.Middleware(next).ServeHTTP(w, r)
			return
		}
		next.ServeHTTP(w, r)
	})
}
