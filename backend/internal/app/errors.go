package app

import (
	"errors"
	"fmt"

	"github.com/hibiken/asynq"

	httpx "github.com/quixgit/greenops-reliabilix/backend/internal/platform/http"

	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/errlog"
)

// HTTPErrorHook turns server-side request failures (5xx, panics) into error-log events of the given service.
func HTTPErrorHook(rec errlog.Recorder, service string) func(httpx.ErrorEvent) {
	return func(e httpx.ErrorEvent) {
		msg := fmt.Sprintf("HTTP %d", e.Status)
		switch {
		case e.Panic && e.Err != nil:
			msg = "panic: " + e.Err.Error()
		case e.Err != nil:
			msg += ": " + e.Err.Error()
		default:
			msg += " (no cause reported by the handler)"
		}
		sev := errlog.Error
		if e.Panic {
			sev = errlog.Critical
		}
		rec.Record(errlog.Event{Service: service, Component: e.Route, Severity: sev, Message: msg, TenantID: e.TenantID, RequestID: e.RequestID})
	}
}

// JobErrorHook turns a failed background job attempt into an error-log event. A job that gave up for good
// (asynq.SkipRetry: denied access, a malformed payload) is usually the customer's side, so it is only a warning.
func JobErrorHook(rec errlog.Recorder, service string) func(taskType string, err error) {
	return func(taskType string, err error) {
		sev := errlog.Error
		if isPermanent(err) {
			sev = errlog.Warning
		}
		rec.Record(errlog.Event{Service: service, Component: taskType, Severity: sev, Message: err.Error()})
	}
}

func isPermanent(err error) bool { return errors.Is(err, asynq.SkipRetry) }
