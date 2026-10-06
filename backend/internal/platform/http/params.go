package httpx

import (
	"errors"
	"net/http"
	"regexp"
	"time"

	"github.com/go-chi/chi/v5"
)

var uuidRe = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

// OptionalUUID reads a UUID query parameter. ok=false means the value was present but malformed.
func OptionalUUID(r *http.Request, name string) (v *string, ok bool) {
	s := r.URL.Query().Get(name)
	if s == "" {
		return nil, true
	}
	if !uuidRe.MatchString(s) {
		return nil, false
	}
	return &s, true
}

// Period reads from/to (YYYY-MM-DD, to exclusive). Default: the last 30 days.
// Windows longer than 400 days are rejected.
func Period(r *http.Request) (from, to time.Time, err error) {
	to = time.Now().UTC().Truncate(24 * time.Hour).Add(24 * time.Hour)
	from = to.AddDate(0, 0, -30)
	if s := r.URL.Query().Get("from"); s != "" {
		if from, err = time.Parse(time.DateOnly, s); err != nil {
			return
		}
	}
	if s := r.URL.Query().Get("to"); s != "" {
		if to, err = time.Parse(time.DateOnly, s); err != nil {
			return
		}
		to = to.AddDate(0, 0, 1)
	}
	if !to.After(from) || to.Sub(from) > 400*24*time.Hour {
		err = errors.New("invalid period")
	}
	return
}

// PathUUID reads a UUID path parameter (chi). ok=false means malformed: respond 404, never touch the DB.
func PathUUID(r *http.Request, name string) (string, bool) {
	v := chi.URLParam(r, name)
	return v, uuidRe.MatchString(v)
}

// ParseUUID reports whether s is a canonical UUID.
func ParseUUID(s string) bool { return uuidRe.MatchString(s) }

// NotFound writes the standard 404 problem.
func NotFound(w http.ResponseWriter, r *http.Request) {
	WriteProblem(w, r, http.StatusNotFound, "not found", "")
}
