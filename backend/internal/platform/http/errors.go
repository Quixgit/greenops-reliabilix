package httpx

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
)

// ErrorEvent describes a server-side failure of a request (a 5xx answer or a panic).
type ErrorEvent struct {
	Route     string // chi route pattern, e.g. "POST /api/v1/reports/{id}/download": no ids, safe to group on
	Status    int
	Err       error // the cause when a handler reported one, a panic wrapped as an error, or nil
	TenantID  string
	RequestID string
	Panic     bool
}

// ErrorHook receives every server-side failure. cmd/api points it at the error log (ADR-0021); it must not block.
var ErrorHook func(ErrorEvent)

// requestFailure is shared by the middleware and the handlers of one request through the context.
type requestFailure struct {
	err    error
	tenant string
}

type failureKey struct{}

func failureOf(ctx context.Context) *requestFailure {
	f, _ := ctx.Value(failureKey{}).(*requestFailure)
	return f
}

// ReportError remembers the cause of a 5xx for the access log and the error log. The first cause wins.
func ReportError(r *http.Request, err error) {
	if f := failureOf(r.Context()); f != nil && f.err == nil {
		f.err = err
	}
}

// SetTenant tells the error log which organization the request belonged to (called by authentication).
func SetTenant(r *http.Request, tenantID string) {
	if f := failureOf(r.Context()); f != nil {
		f.tenant = tenantID
	}
}

// Internal answers 500 without internals and records the cause server-side. Use it instead of writing the
// problem directly, so unexpected errors are never swallowed.
func Internal(w http.ResponseWriter, r *http.Request, err error) {
	ReportError(r, err)
	WriteProblem(w, r, http.StatusInternalServerError, "internal error", "")
}

func routeOf(r *http.Request) string {
	if rc := chi.RouteContext(r.Context()); rc != nil {
		if p := rc.RoutePattern(); p != "" {
			return r.Method + " " + p
		}
	}
	return r.Method + " (unmatched)"
}

func reportPanic(r *http.Request, rec any) {
	if ErrorHook == nil {
		return
	}
	e := ErrorEvent{Route: routeOf(r), Status: http.StatusInternalServerError, Panic: true, RequestID: RequestID(r.Context())}
	if f := failureOf(r.Context()); f != nil {
		e.TenantID = f.tenant
	}
	if err, ok := rec.(error); ok {
		e.Err = err
	} else {
		e.Err = errors.New(fmt.Sprint(rec))
	}
	ErrorHook(e)
}
