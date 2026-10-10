package billing

import (
	"context"
	"net/http"
	"strings"
)

// PaymentLink sells through a payment link the operator created at any service that offers one: a Payoneer payment
// request, a Wise or PayPal.me link, a bank's hosted page. There is no callback, so a payment is not confirmed by the
// platform: the customer is sent to the link, the request is recorded for the operator, and the operator assigns the plan
// with its period in the console once the money arrived. (A direct Payoneer Checkout adapter needs a merchant account
// and its API credentials and is a separate piece of work.)
type PaymentLink struct{ Values Values }

func (PaymentLink) ID() string { return "paymentlink" }

func (p PaymentLink) Name() string {
	// The label is what the customer recognises: "Payoneer", "Wise" ...
	if l := strings.TrimSpace(p.Values(context.Background(), "paymentlink", "label")); l != "" {
		return l + " (payment link)"
	}
	return "Payment link"
}

func (p PaymentLink) val(ctx context.Context, k string) string {
	return p.Values(ctx, "paymentlink", k)
}

func (p PaymentLink) Configured(ctx context.Context) bool { return len(p.Plans(ctx)) > 0 }

func (p PaymentLink) Plans(ctx context.Context) []string {
	var out []string
	for _, pl := range []string{"team", "business", "enterprise"} {
		if p.val(ctx, "url_"+pl) != "" {
			out = append(out, pl)
		}
	}
	return out
}

// Manual: the platform does not learn that the payment happened; the operator confirms it.
func (PaymentLink) Manual() bool { return true }

func (p PaymentLink) Checkout(ctx context.Context, req CheckoutRequest) (CheckoutResult, error) {
	u := p.val(ctx, "url_"+req.Plan)
	if u == "" {
		return CheckoutResult{}, ErrUnknownPlan
	}
	msg := "After you pay, the plan is activated by our team (usually within one business day)."
	if i := strings.TrimSpace(p.val(ctx, "instructions")); i != "" {
		msg = i
	}
	return CheckoutResult{URL: u, Message: msg}, nil
}

func (PaymentLink) ParseWebhook(context.Context, *http.Request, []byte) (*Event, error) {
	return nil, nil
}

// Manual is implemented by providers whose payments the operator confirms by hand.
type Manual interface{ Manual() bool }
