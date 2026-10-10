// Package billing is the payment provider port (ADR-0038). Providers hold the money and the card data; the platform
// keeps only an identifier and the resulting subscription state. Each provider is an adapter that can start a hosted
// checkout for an organization and turn the provider's webhook into a normalized Event. Credentials come from the
// integrations store (the console), never from the environment at the call site.
package billing

import (
	"context"
	"errors"
	"net/http"
	"sort"
	"time"
)

// Values reads one console-managed value of an integration ("" when not set).
type Values func(ctx context.Context, integration, key string) string

// CheckoutRequest is what a provider needs to start a payment for an organization.
type CheckoutRequest struct {
	OrganizationID   string
	OrganizationName string
	Email            string
	Plan             string // plan id of the catalog
	ReturnURL        string // where the provider sends the person afterwards
}

// CheckoutResult is either a URL to send the person to, or a message when the provider has no hosted page (invoice).
type CheckoutResult struct {
	URL     string `json:"url,omitempty"`
	Message string `json:"message,omitempty"`
}

// Event is a provider notification reduced to what the platform acts on.
type Event struct {
	ID             string // unique per provider; the platform applies an ID once
	OrganizationID string
	Plan           string // plan id; "free" when the subscription ended
	Status         string // active | trialing | past_due | canceled
	PeriodEnd      *time.Time
	Reference      string // the provider's own id, for support
}

var (
	// ErrNotConfigured: the provider has no credentials in the console.
	ErrNotConfigured = errors.New("billing provider is not configured")
	// ErrBadSignature: the webhook could not be verified; it must be ignored.
	ErrBadSignature = errors.New("webhook signature is not valid")
	// ErrUnknownPlan: the plan has no price at this provider.
	ErrUnknownPlan = errors.New("the plan has no price at this provider")
)

// Provider is one payment provider.
type Provider interface {
	ID() string
	Name() string
	// Configured reports whether the credentials needed to take a payment are set.
	Configured(ctx context.Context) bool
	// Plans lists the plan ids that have a price at this provider.
	Plans(ctx context.Context) []string
	Checkout(ctx context.Context, req CheckoutRequest) (CheckoutResult, error)
	// ParseWebhook verifies and reduces a notification. A nil Event with a nil error is a notification to ignore.
	ParseWebhook(ctx context.Context, r *http.Request, body []byte) (*Event, error)
}

// Acknowledger is implemented by providers that require a specific JSON answer to a notification (WayForPay).
type Acknowledger interface {
	Ack(ctx context.Context, body []byte) any
}

// Registry holds the providers the platform knows.
type Registry struct{ byID map[string]Provider }

func NewRegistry(ps ...Provider) *Registry {
	r := &Registry{byID: map[string]Provider{}}
	for _, p := range ps {
		r.byID[p.ID()] = p
	}
	return r
}

func (r *Registry) Get(id string) (Provider, bool) { p, ok := r.byID[id]; return p, ok }

// All lists the providers sorted by id.
func (r *Registry) All() []Provider {
	out := make([]Provider, 0, len(r.byID))
	for _, p := range r.byID {
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID() < out[j].ID() })
	return out
}
