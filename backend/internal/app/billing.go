package app

import (
	"context"

	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/billing"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/integrations"
)

// buildBilling wires the payment providers. Their credentials are read from the integrations store on every use, so a
// key set in the console applies without a restart (ADR-0029).
func buildBilling(ints *integrations.Store) *billing.Registry {
	values := func(ctx context.Context, integration, key string) string {
		if ints == nil {
			return ""
		}
		return ints.Value(ctx, integration, key)
	}
	return billing.NewRegistry(
		billing.Stripe{Values: values},
		billing.Paddle{Values: values},
		billing.LiqPay{Values: values},
		billing.WayForPay{Values: values},
		billing.Invoice{Values: values},
		billing.PaymentLink{Values: values},
	)
}
