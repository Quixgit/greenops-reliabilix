package httpx

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestMaxBodyForRaisesTheLimitOnlyForTheNamedPostRoute(t *testing.T) {
	big := bytes.Repeat([]byte("x"), 3000)
	h := MaxBodyFor(1000, map[string]int64{"/upload": 5000})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := io.ReadAll(r.Body); err != nil {
			w.WriteHeader(http.StatusRequestEntityTooLarge)
		}
	}))
	for _, c := range []struct {
		method, path string
		want         int
	}{{"POST", "/upload", 200}, {"POST", "/other", 413}, {"GET", "/upload", 413}, {"POST", "/upload/x", 413}} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(c.method, c.path, bytes.NewReader(big)))
		if rec.Code != c.want {
			t.Errorf("%s %s: %d, want %d", c.method, c.path, rec.Code, c.want)
		}
	}
}
