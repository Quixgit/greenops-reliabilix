package billing

import (
	"context"
	"net/http"
)

// Invoice is the provider for companies that pay by bank transfer: no hosted page and no webhook. A request is recorded
// for the operator, who sends the invoice and, once it is paid, assigns the plan with its period in the console.
type Invoice struct{ Values Values }

func (Invoice) ID() string   { return "invoice" }
func (Invoice) Name() string { return "Invoice (bank transfer)" }

// Configured: usable once a billing contact is set, so a customer is told where the invoice comes from.
func (i Invoice) Configured(ctx context.Context) bool {
	return i.Values(ctx, "invoice", "contact") != ""
}

func (Invoice) Plans(context.Context) []string { return []string{"team", "business", "enterprise"} }

func (i Invoice) Checkout(ctx context.Context, req CheckoutRequest) (CheckoutResult, error) {
	contact := i.Values(ctx, "invoice", "contact")
	if contact == "" {
		return CheckoutResult{}, ErrNotConfigured
	}
	return CheckoutResult{Message: "Your request for the " + req.Plan + " plan is recorded. We will send an invoice to " + req.Email + ". Questions: " + contact + "."}, nil
}

func (Invoice) ParseWebhook(context.Context, *http.Request, []byte) (*Event, error) { return nil, nil }

// Manual: an invoice is confirmed by the operator once it is paid.
func (Invoice) Manual() bool { return true }
