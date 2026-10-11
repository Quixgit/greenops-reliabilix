// Package notify is the port every domain uses to tell people something happened. A domain builds an Event and
// calls Notifier.Publish; it knows nothing about channels (e-mail, Slack, webhooks ...), destinations or retries.
// The notifications module implements the port and is wired in internal/app, so adding a channel never touches
// a producer, and a producer never touches a channel (ADR-0019).
package notify

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// Severity orders events so a destination can subscribe to "warning and above".
type Severity string

const (
	Info     Severity = "info"
	Warning  Severity = "warning"
	Critical Severity = "critical"
)

// Rank is 1 (info) to 3 (critical); 0 for an unknown value.
func (s Severity) Rank() int {
	switch s {
	case Info:
		return 1
	case Warning:
		return 2
	case Critical:
		return 3
	}
	return 0
}

// Event types. Convention: <subject>.<what happened>. Add a type here when a producer starts publishing it;
// destinations can subscribe to any of these or to "*".
const (
	TypeReportReady            = "report.ready"
	TypeReportFailed           = "report.failed"
	TypeConnectionError        = "connection.error"
	TypeConnectionDataQuality  = "connection.data_quality"
	TypeRecommendationVerified = "recommendation.verified"
	TypeAlertOpened            = "alert.opened"
	TypeAutomationApproved     = "automation.plan_approved"
	TypeAutomationResult       = "automation.result_reported"
	TypeNotificationTest       = "notification.test" // only ever sent by the "send a test" action
)

// Wildcard subscribes a destination to every event type.
const Wildcard = "*"

// TypeInfo describes one publishable event type (served by the API so a UI can list what can be subscribed to).
type TypeInfo struct {
	Type        string   `json:"type"`
	Description string   `json:"description"`
	Severity    Severity `json:"default_severity"`
}

// Catalog lists the event types producers publish today.
func Catalog() []TypeInfo {
	return []TypeInfo{
		{TypeReportReady, "A requested report has been generated and can be downloaded", Info},
		{TypeReportFailed, "A requested report could not be generated", Warning},
		{TypeConnectionError, "A cloud connection needs attention (access was denied or validation failed)", Warning},
		{TypeConnectionDataQuality, "Stored cost data differs from the totals the cloud reports for some days", Warning},
		{TypeAlertOpened, "A budget or connection alert was opened", Warning},
		{TypeRecommendationVerified, "The measured result of an applied recommendation is available", Info},
		{TypeAutomationApproved, "A change plan was approved and is ready to be applied", Info},
		{TypeAutomationResult, "The result of applying a change plan was reported (applied, failed or rolled back)", Info},
	}
}

// KnownType reports whether t can be subscribed to.
func KnownType(t string) bool {
	if t == Wildcard {
		return true
	}
	for _, c := range Catalog() {
		if c.Type == t {
			return true
		}
	}
	return false
}

// Event is channel-neutral. Title and Body are plain text: channels escape them for their own format.
type Event struct {
	Type     string            `json:"type"`
	Severity Severity          `json:"severity"`
	Title    string            `json:"title"`
	Body     string            `json:"body,omitempty"`
	Path     string            `json:"-"`               // app-relative link ("/reports"); the notifications module makes it absolute
	Link     string            `json:"link,omitempty"`  // absolute URL, set by the notifications module
	Attrs    map[string]string `json:"attrs,omitempty"` // small structured facts (ids), never secrets or personal data
	// DedupeKey makes Publish idempotent per destination: the same key is delivered at most once. Empty = no dedupe.
	DedupeKey string `json:"-"`
}

const (
	maxTitle = 200
	maxBody  = 2000
	maxAttrs = 20
)

var ErrInvalidEvent = errors.New("invalid notification event")

// Validate bounds the event so one producer cannot flood a channel.
func (e Event) Validate() error {
	switch {
	case e.Type != TypeNotificationTest && (!KnownType(e.Type) || e.Type == Wildcard):
		return fmt.Errorf("%w: unknown type %q", ErrInvalidEvent, e.Type)
	case e.Severity.Rank() == 0:
		return fmt.Errorf("%w: unknown severity %q", ErrInvalidEvent, e.Severity)
	case strings.TrimSpace(e.Title) == "" || len(e.Title) > maxTitle:
		return fmt.Errorf("%w: title must be 1-%d bytes", ErrInvalidEvent, maxTitle)
	case len(e.Body) > maxBody:
		return fmt.Errorf("%w: body is over %d bytes", ErrInvalidEvent, maxBody)
	case len(e.Attrs) > maxAttrs:
		return fmt.Errorf("%w: more than %d attrs", ErrInvalidEvent, maxAttrs)
	case e.Path != "" && !strings.HasPrefix(e.Path, "/"):
		return fmt.Errorf("%w: path must be app-relative", ErrInvalidEvent)
	}
	return nil
}

// Notifier is what producers depend on. Publish is best-effort from the producer's point of view: a failure to
// notify must never fail the business operation, so callers log the error and carry on.
type Notifier interface {
	Publish(ctx context.Context, tenantID string, e Event) error
}

// Noop drops every event (tests, and processes where notifications are not configured).
type Noop struct{}

func (Noop) Publish(context.Context, string, Event) error { return nil }

// Recorder collects events in memory (tests).
type Recorder struct{ Events []Event }

func (r *Recorder) Publish(_ context.Context, _ string, e Event) error {
	r.Events = append(r.Events, e)
	return nil
}
