// Package domain defines notification destinations, deliveries and the Channel contract every delivery
// mechanism implements. It has no HTTP, SQL, queue or SDK imports: channels live in ../channels.
package domain

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/notify"
)

// Kind names a channel: the key of the Registry and the destinations.kind column.
type Kind string

const (
	KindEmail   Kind = "email"
	KindWebhook Kind = "webhook"
	KindSlack   Kind = "slack"
)

var (
	ErrNotFound      = errors.New("not found")
	ErrInvalid       = errors.New("invalid destination")
	ErrNotConfigured = errors.New("notifications are not configured on this deployment")
	ErrLimit         = errors.New("destination limit reached")
	ErrUnknownKind   = errors.New("unknown or unavailable channel")
)

// MaxDestinationsPerTenant bounds fan-out per event.
const MaxDestinationsPerTenant = 20

// Destination is where a tenant wants events delivered. The address itself is secret and never leaves the
// repository/application layers: callers only see Hint.
type Destination struct {
	ID          string          `json:"id"`
	Kind        Kind            `json:"kind"`
	Name        string          `json:"name"`
	Hint        string          `json:"target_hint"`
	EventTypes  []string        `json:"event_types"`
	MinSeverity notify.Severity `json:"min_severity"`
	Enabled     bool            `json:"enabled"`
	CreatedBy   string          `json:"created_by"`
	CreatedAt   time.Time       `json:"created_at"`

	// SealedTarget is the encrypted address (see platform/secretbox). Never serialised.
	SealedTarget string `json:"-"`
}

// Validate checks the tenant-supplied settings (the address is validated by the channel).
func (d Destination) Validate() error {
	if n := strings.TrimSpace(d.Name); n == "" || len(n) > 100 {
		return fmt.Errorf("%w: name must be 1-100 characters", ErrInvalid)
	}
	if d.MinSeverity.Rank() == 0 {
		return fmt.Errorf("%w: min_severity must be info, warning or critical", ErrInvalid)
	}
	if len(d.EventTypes) == 0 || len(d.EventTypes) > 50 {
		return fmt.Errorf("%w: event_types must list 1-50 types", ErrInvalid)
	}
	for _, t := range d.EventTypes {
		if !notify.KnownType(t) {
			return fmt.Errorf("%w: unknown event type %q", ErrInvalid, t)
		}
	}
	return nil
}

// Matches reports whether the destination wants the event.
func (d Destination) Matches(e notify.Event) bool {
	if !d.Enabled || e.Severity.Rank() < d.MinSeverity.Rank() {
		return false
	}
	return slices.Contains(d.EventTypes, notify.Wildcard) || slices.Contains(d.EventTypes, e.Type)
}

type Status string

const (
	Pending Status = "pending"
	Sent    Status = "sent"
	Failed  Status = "failed"
	Skipped Status = "skipped" // the destination was removed before the delivery could be sent
)

// Delivery is one event for one destination: a row of the outbox.
type Delivery struct {
	ID              string          `json:"id"`
	DestinationID   *string         `json:"destination_id"`
	Kind            Kind            `json:"kind"`
	DestinationName string          `json:"destination_name"`
	EventType       string          `json:"event_type"`
	Severity        notify.Severity `json:"severity"`
	Status          Status          `json:"status"`
	Attempts        int             `json:"attempts"`
	NextAttemptAt   time.Time       `json:"next_attempt_at"`
	LastError       *string         `json:"last_error"`
	CreatedAt       time.Time       `json:"created_at"`
	SentAt          *time.Time      `json:"sent_at"`

	Event notify.Event `json:"-"`
}

// Message is what a channel sends: the event plus delivery metadata.
type Message struct {
	DeliveryID string
	TenantID   string
	Attempt    int
	CreatedAt  time.Time
	Event      notify.Event
}

// Target is a destination as a channel sees it at send time (address decrypted for this call only).
type Target struct {
	DestinationID string
	Address       string
	// Secret is a per-destination secret derived from the platform key (never stored); channels that sign
	// payloads use it.
	Secret string
}

// Channel is one delivery mechanism. To add Teams, PagerDuty, SMS ... implement it in ../channels and register
// it in internal/app: no producer, table or migration changes.
type Channel interface {
	Kind() Kind
	// Prepare validates and normalises an address supplied by a tenant and returns a non-secret hint for display
	// ("a***@example.com", "hooks.slack.com").
	Prepare(address string) (normalized, hint string, err error)
	// Send delivers one message. Return Permanent(err) when retrying cannot help (rejected address, 4xx).
	Send(ctx context.Context, t Target, m Message) error
}

// AvailableChannel is implemented by channels that may be configured at run time (e-mail): when it reports false the
// channel is hidden from the catalog and refused for new destinations.
type AvailableChannel interface {
	Available(ctx context.Context) bool
}

// PersonalChannel is implemented by channels whose address identifies a person (e-mail). The platform sends
// to such addresses only when they belong to a member of the tenant, so it cannot be used to mail strangers.
type PersonalChannel interface{ PersonalAddress() bool }

// SigningChannel is implemented by channels that sign what they send (webhook). The application derives a
// per-destination secret for every channel; for these channels it is also shown to the tenant once, on creation.
type SigningChannel interface{ UsesSigningSecret() bool }

// Registry holds the channels available in this process.
type Registry struct{ byKind map[Kind]Channel }

func NewRegistry(cs ...Channel) *Registry {
	r := &Registry{byKind: map[Kind]Channel{}}
	for _, c := range cs {
		r.byKind[c.Kind()] = c
	}
	return r
}

func (r *Registry) Get(k Kind) (Channel, error) {
	if r != nil {
		if c, ok := r.byKind[k]; ok {
			return c, nil
		}
	}
	return nil, ErrUnknownKind
}

// Kinds lists the available channels in a stable order.
// KindsAvailable is Kinds without the channels that report themselves unavailable right now.
func (r *Registry) KindsAvailable(ctx context.Context) []Kind {
	var out []Kind
	for _, k := range r.Kinds() {
		if c, err := r.Get(k); err == nil {
			if a, ok := c.(AvailableChannel); ok && !a.Available(ctx) {
				continue
			}
		}
		out = append(out, k)
	}
	return out
}

func (r *Registry) Kinds() []Kind {
	out := []Kind{}
	if r != nil {
		for k := range r.byKind {
			out = append(out, k)
		}
	}
	slices.Sort(out)
	return out
}

type permanentError struct{ err error }

func (p permanentError) Error() string { return p.err.Error() }
func (p permanentError) Unwrap() error { return p.err }

// Permanent marks a send error as final: the delivery is failed without further attempts.
func Permanent(err error) error {
	if err == nil {
		return nil
	}
	return permanentError{err}
}

// IsPermanent reports whether err (or anything it wraps) was marked Permanent.
func IsPermanent(err error) bool {
	var p permanentError
	return errors.As(err, &p)
}

// Retry policy: attempt n (1-based) failing transiently is retried after 30s * 2^(n-1), capped at one hour.
const MaxAttempts = 8

func Backoff(attempt int) time.Duration { return BackoffFrom(30*time.Second, attempt) }

// BackoffFrom is Backoff with a configurable first delay (settings: notifications.retry_base_seconds).
func BackoffFrom(base time.Duration, attempt int) time.Duration {
	if attempt < 1 {
		attempt = 1
	}
	d := base << min(attempt-1, 10)
	return min(d, time.Hour)
}

// Repository is the persistence port.
type Repository interface {
	CountDestinations(ctx context.Context, tenantID string) (int, error)
	// InsertDestination stores d with the ID and SealedTarget the caller already set (the ID is the encryption context).
	InsertDestination(ctx context.Context, tenantID string, d Destination) (Destination, error)
	ListDestinations(ctx context.Context, tenantID string) ([]Destination, error)
	GetDestination(ctx context.Context, tenantID, id string) (Destination, error)
	SetEnabled(ctx context.Context, tenantID, id string, enabled bool) (Destination, error)
	DeleteDestination(ctx context.Context, tenantID, id string) error

	// InsertDeliveries stores one pending delivery per destination (idempotent per dedupe key) and returns the
	// ids that are new.
	InsertDeliveries(ctx context.Context, tenantID string, ds []Destination, e notify.Event) ([]string, error)
	ListDeliveries(ctx context.Context, tenantID string, limit int) ([]Delivery, error)

	// Claim takes the lease on a due delivery and increments attempts. ok=false means: nothing to do (already
	// sent, not due, or being sent by someone else).
	Claim(ctx context.Context, tenantID, deliveryID string, lease time.Duration) (d Delivery, dest *Destination, ok bool, err error)
	MarkSent(ctx context.Context, tenantID, deliveryID string) error
	MarkRetry(ctx context.Context, tenantID, deliveryID string, next time.Time, reason string) error
	MarkFailed(ctx context.Context, tenantID, deliveryID string, status Status, reason string) error

	// Cross-tenant maintenance (SECURITY DEFINER functions).
	ListDue(ctx context.Context, limit int) ([]DueRef, error)
	Purge(ctx context.Context, retain time.Duration) (int64, error)
}

// DueRef identifies a delivery the sweep should queue.
type DueRef struct{ TenantID, DeliveryID string }

// InboxItem is one message in the in-product inbox (the bell).
type InboxItem struct {
	ID        string          `json:"id"`
	Type      string          `json:"type"`
	Severity  notify.Severity `json:"severity"`
	Title     string          `json:"title"`
	Body      string          `json:"body"`
	Path      string          `json:"path"`
	CreatedAt time.Time       `json:"created_at"`
	Unread    bool            `json:"unread"`
}
