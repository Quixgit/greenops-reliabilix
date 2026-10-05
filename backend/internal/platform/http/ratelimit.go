package httpx

import (
	"net"
	"net/http"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// RateLimit is a per-client token bucket (client = remote IP). It is
// in-process; move to Redis when running several API replicas.
func RateLimit(perSec float64, burst int) Middleware {
	type entry struct {
		l    *rate.Limiter
		seen time.Time
	}
	var (
		mu sync.Mutex
		m  = map[string]*entry{}
	)
	go func() {
		for range time.Tick(5 * time.Minute) {
			mu.Lock()
			for k, e := range m {
				if time.Since(e.seen) > 10*time.Minute {
					delete(m, k)
				}
			}
			mu.Unlock()
		}
	}()
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip, _, err := net.SplitHostPort(r.RemoteAddr)
			if err != nil {
				ip = r.RemoteAddr
			}
			mu.Lock()
			e, ok := m[ip]
			if !ok {
				e = &entry{l: rate.NewLimiter(rate.Limit(perSec), burst)}
				m[ip] = e
			}
			e.seen = time.Now()
			allowed := e.l.Allow()
			mu.Unlock()
			if !allowed {
				w.Header().Set("Retry-After", "1")
				WriteProblem(w, r, http.StatusTooManyRequests, "rate limit exceeded", "")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// CORS allows only the listed origins (exact match). Empty list = no CORS.
func CORS(origins []string) Middleware {
	allowed := map[string]bool{}
	for _, o := range origins {
		allowed[o] = true
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if o := r.Header.Get("Origin"); o != "" && allowed[o] {
				h := w.Header()
				h.Set("Access-Control-Allow-Origin", o)
				h.Set("Vary", "Origin")
				h.Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
				h.Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE")
				if r.Method == http.MethodOptions {
					w.WriteHeader(http.StatusNoContent)
					return
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}
