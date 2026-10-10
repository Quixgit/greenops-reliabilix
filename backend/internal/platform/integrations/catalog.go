// Package integrations holds the keys and addresses of the external services the platform talks to, and lets an
// operator manage them in the console (ADR-0029). A value comes from the database when an operator set one, else from
// the deployment environment, else it is absent. Secret values are sealed at rest and never returned to a client.
package integrations

import (
	"errors"
	"fmt"
	"net/mail"
	"regexp"
	"strconv"
	"strings"
)

// Field is one setting of an integration.
type Field struct {
	Key         string `json:"key"`
	Label       string `json:"label"`
	Help        string `json:"help,omitempty"`
	Placeholder string `json:"placeholder,omitempty"`
	Secret      bool   `json:"secret"`
	Required    bool   `json:"required"`
	Env         string `json:"env"` // the environment variable that provides the value when none is set in the console
	Default     string `json:"default,omitempty"`
	validate    func(string) error
}

// Integration is one external service.
type Integration struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Description string  `json:"description"`
	Fields      []Field `json:"fields"`
	// Testable integrations have a connection test in the console.
	Testable bool `json:"testable"`
	// Startup integrations are read when a service starts (a client is built once from them): a change applies at the
	// next restart, which the console says.
	Startup bool `json:"startup"`
	// Prepared integrations have their settings ready so only the credential has to be added, but no platform code
	// reads them yet. The console says so; nothing is claimed to work.
	Prepared bool `json:"prepared"`
}

var (
	zonePair = regexp.MustCompile(`^[a-z0-9-]{2,40}=[A-Za-z0-9-]{2,40}$`)
	ErrField = errors.New("invalid value")
)

func validZoneOverrides(v string) error {
	for _, p := range strings.Split(v, ",") {
		if !zonePair.MatchString(strings.TrimSpace(p)) {
			return fmt.Errorf("%w: use region=ZONE pairs separated by commas (eu-central-1=DE)", ErrField)
		}
	}
	return nil
}

func validSecret(min int) func(string) error {
	return func(v string) error {
		if len(v) < min || len(v) > 500 || strings.ContainsAny(v, " \t\r\n") {
			return fmt.Errorf("%w: expected %d-500 characters without spaces", ErrField, min)
		}
		return nil
	}
}

var hostRe = regexp.MustCompile(`^[A-Za-z0-9]([A-Za-z0-9.-]{0,251}[A-Za-z0-9])?$`)

func validHost(v string) error {
	if !hostRe.MatchString(v) {
		return fmt.Errorf("%w: a host name such as smtp.example.com", ErrField)
	}
	return nil
}

func validPort(v string) error {
	n, err := strconv.Atoi(v)
	if err != nil || n < 1 || n > 65535 {
		return fmt.Errorf("%w: a port from 1 to 65535", ErrField)
	}
	return nil
}

func validFrom(v string) error {
	if _, err := mail.ParseAddress(v); err != nil || len(v) > 254 || strings.ContainsAny(v, "\r\n") {
		return fmt.Errorf("%w: an address such as Reliabilix <notifications@example.com>", ErrField)
	}
	return nil
}

func validUser(v string) error {
	if len(v) > 254 || strings.ContainsAny(v, "\r\n") {
		return fmt.Errorf("%w: too long or contains a line break", ErrField)
	}
	return nil
}

var (
	awsAccount = regexp.MustCompile(`^[0-9]{12}$`)
	gcpProject = regexp.MustCompile(`^[a-z][a-z0-9-]{4,28}[a-z0-9]$`)
	gcpSA      = regexp.MustCompile(`^[a-z0-9-]{6,30}@[a-z][a-z0-9-]{4,28}[a-z0-9]\.iam\.gserviceaccount\.com$`)
	region     = regexp.MustCompile(`^[a-z0-9-]{2,40}$`)
	bucket     = regexp.MustCompile(`^[a-z0-9][a-z0-9.-]{1,61}[a-z0-9]$`)
)

func match(re *regexp.Regexp, what string) func(string) error {
	return func(v string) error {
		if !re.MatchString(v) {
			return fmt.Errorf("%w: %s", ErrField, what)
		}
		return nil
	}
}

var paytypesRe = regexp.MustCompile(`^(card|apay|gpay|liqpay|privat24|qr|invoice)(,(card|apay|gpay|liqpay|privat24|qr|invoice))*$`)

func validPaytypes(v string) error {
	if !paytypesRe.MatchString(v) {
		return fmt.Errorf("%w: comma-separated from card, apay, gpay, liqpay, privat24, qr, invoice", ErrField)
	}
	return nil
}

func validNote(v string) error {
	if len(v) > 500 || strings.ContainsAny(v, "\r\n") {
		return fmt.Errorf("%w: up to 500 characters on one line", ErrField)
	}
	return nil
}

var (
	refRe    = regexp.MustCompile(`^[A-Za-z0-9_.:-]{3,120}$`)
	amountRe = regexp.MustCompile(`^[0-9]{1,7}(\.[0-9]{1,2})?$`)
)

// validRef accepts a provider's identifier (a price id, a public key).
func validRef(v string) error {
	if !refRe.MatchString(v) {
		return fmt.Errorf("%w: letters, digits and . _ : - (3-120 characters)", ErrField)
	}
	return nil
}

func validAmount(v string) error {
	if !amountRe.MatchString(v) || v == "0" {
		return fmt.Errorf("%w: an amount such as 1999 or 1999.50", ErrField)
	}
	return nil
}

func validContact(v string) error {
	if _, err := mail.ParseAddress(v); err != nil || len(v) > 254 {
		return fmt.Errorf("%w: an e-mail address", ErrField)
	}
	return nil
}

func validURL(v string) error {
	if len(v) > 300 || !(strings.HasPrefix(v, "https://") || strings.HasPrefix(v, "http://")) || strings.ContainsAny(v, " \t\r\n") {
		return fmt.Errorf("%w: a full URL starting with https://", ErrField)
	}
	return nil
}

func validBool(v string) error {
	if v != "true" && v != "false" {
		return fmt.Errorf("%w: true or false", ErrField)
	}
	return nil
}

// Catalog lists the integrations an operator can manage in the console. Services that are fixed by the deployment
// (identity provider, object storage, error tracking) are reported by the system view, not edited here.
func Catalog() []Integration {
	return []Integration{
		{
			ID: "electricitymaps", Name: "Electricity Maps", Testable: true,
			Description: "Grid carbon intensity for the carbon engine and the CI gate. Without a key only stored readings are used.",
			Fields: []Field{
				{Key: "api_key", Label: "API key", Secret: true, Required: true, Env: "ELECTRICITYMAPS_API_KEY", Help: "From the Electricity Maps dashboard. Stored sealed, shown only as configured.", validate: validSecret(8)},
				{Key: "zone_overrides", Label: "Zone overrides", Env: "ELECTRICITYMAPS_ZONE_OVERRIDES", Placeholder: "eu-central-1=DE,us-east-1=US-MIDA-PJM", Help: "Replace or add region to zone mappings. Optional.", validate: validZoneOverrides},
			},
		},
		{
			ID: "watttime", Name: "WattTime", Prepared: true,
			Description: "Marginal operating emissions (MOER) as a second grid-data source. Prepared: the credentials are stored sealed, and the connector switches on without another configuration step once it is released.",
			Fields: []Field{
				{Key: "username", Label: "User name", Required: true, Env: "WATTTIME_USERNAME", Help: "The WattTime account user name.", validate: validUser},
				{Key: "password", Label: "Password", Secret: true, Required: true, Env: "WATTTIME_PASSWORD", Help: "Stored sealed, shown only as configured.", validate: validSecret(1)},
			},
		},
		{
			ID: "uk_carbon_intensity", Name: "UK Carbon Intensity API", Prepared: true,
			Description: "Free grid carbon intensity for Great Britain. It needs no key; the address can be changed for a mirror or a test double. Prepared: not read by the platform yet.",
			Fields: []Field{
				{Key: "base_url", Label: "API address", Env: "UKCI_BASE_URL", Default: "https://api.carbonintensity.org.uk", Placeholder: "https://api.carbonintensity.org.uk", validate: validURL},
			},
		},
		{
			ID: "stripe", Name: "Stripe (payments)",
			Description: "Card payments and subscriptions worldwide, with Apple Pay and Google Pay (turn them on in the Stripe dashboard and register this site's domain for Apple Pay). Customers pay on Stripe's hosted checkout; the platform keeps only the subscription state. Point Stripe's webhook at /api/v1/billing/webhooks/stripe (events customer.subscription.created, updated, deleted).",
			Fields: []Field{
				{Key: "secret_key", Label: "Secret key", Secret: true, Required: true, Env: "STRIPE_SECRET_KEY", Placeholder: "sk_live_…", Help: "A restricted key with write access to Checkout Sessions is enough.", validate: validSecret(8)},
				{Key: "webhook_secret", Label: "Webhook signing secret", Secret: true, Required: true, Env: "STRIPE_WEBHOOK_SECRET", Placeholder: "whsec_…", validate: validSecret(8)},
				{Key: "price_team", Label: "Price id: Team", Env: "STRIPE_PRICE_TEAM", Placeholder: "price_…", Help: "A recurring price. A plan without a price cannot be bought here.", validate: validRef},
				{Key: "price_business", Label: "Price id: Business", Env: "STRIPE_PRICE_BUSINESS", Placeholder: "price_…", validate: validRef},
			},
		},
		{
			ID: "paddle", Name: "Paddle (payments)",
			Description: "A merchant of record: Paddle also collects and pays sales tax and VAT for the customer's country, which is convenient for selling subscriptions abroad. Apple Pay and Google Pay are offered on its checkout when enabled in Paddle. Point Paddle's notification destination at /api/v1/billing/webhooks/paddle (subscription events).",
			Fields: []Field{
				{Key: "api_key", Label: "API key", Secret: true, Required: true, Env: "PADDLE_API_KEY", validate: validSecret(8)},
				{Key: "webhook_secret", Label: "Notification secret key", Secret: true, Required: true, Env: "PADDLE_WEBHOOK_SECRET", validate: validSecret(8)},
				{Key: "price_team", Label: "Price id: Team", Env: "PADDLE_PRICE_TEAM", Placeholder: "pri_…", validate: validRef},
				{Key: "price_business", Label: "Price id: Business", Env: "PADDLE_PRICE_BUSINESS", Placeholder: "pri_…", validate: validRef},
				{Key: "sandbox", Label: "Sandbox", Env: "PADDLE_SANDBOX", Default: "false", Help: "true to use Paddle's sandbox while testing.", validate: validBool},
			},
		},
		{
			ID: "liqpay", Name: "LiqPay (payments, Ukraine)",
			Description: "PrivatBank's payment service: cards, Apple Pay, Google Pay and Privat24 in hryvnia, with monthly subscriptions. Set the callback address to /api/v1/billing/webhooks/liqpay on this deployment.",
			Fields: []Field{
				{Key: "public_key", Label: "Public key", Required: true, Env: "LIQPAY_PUBLIC_KEY", Placeholder: "sandbox_i… or i…", validate: validRef},
				{Key: "private_key", Label: "Private key", Secret: true, Required: true, Env: "LIQPAY_PRIVATE_KEY", validate: validSecret(8)},
				{Key: "amount_team", Label: "Monthly amount: Team, UAH", Env: "LIQPAY_AMOUNT_TEAM", Placeholder: "1999", Help: "A plan without an amount cannot be bought here.", validate: validAmount},
				{Key: "amount_business", Label: "Monthly amount: Business, UAH", Env: "LIQPAY_AMOUNT_BUSINESS", Placeholder: "5999", validate: validAmount},
				{Key: "callback_url", Label: "Callback address", Env: "LIQPAY_CALLBACK_URL", Placeholder: "https://api.example.com/api/v1/billing/webhooks/liqpay", Help: "The public address LiqPay calls after a payment.", validate: validURL},
				{Key: "paytypes", Label: "Payment methods", Env: "LIQPAY_PAYTYPES", Default: "card,apay,gpay", Placeholder: "card,apay,gpay,privat24", Help: "Comma-separated: card, apay (Apple Pay), gpay (Google Pay), privat24, qr. The wallets must also be enabled in the LiqPay business account.", validate: validPaytypes},
				{Key: "period_days", Label: "Days a payment buys", Env: "LIQPAY_PERIOD_DAYS", Default: "30", validate: validPort},
			},
		},
		{
			ID: "wayforpay", Name: "WayForPay (payments, Ukraine)",
			Description: "Cards, Apple Pay, Google Pay and instalments in hryvnia on a hosted invoice page (the methods enabled in the WayForPay account). One payment buys a period of the plan. Set the service address to /api/v1/billing/webhooks/wayforpay on this deployment.",
			Fields: []Field{
				{Key: "merchant_account", Label: "Merchant account", Required: true, Env: "WAYFORPAY_MERCHANT_ACCOUNT", validate: validRef},
				{Key: "secret_key", Label: "Merchant secret key", Secret: true, Required: true, Env: "WAYFORPAY_SECRET_KEY", validate: validSecret(8)},
				{Key: "merchant_domain", Label: "Merchant domain", Required: true, Env: "WAYFORPAY_MERCHANT_DOMAIN", Placeholder: "app.example.com", Help: "The domain registered with WayForPay.", validate: validHost},
				{Key: "amount_team", Label: "Amount per period: Team, UAH", Env: "WAYFORPAY_AMOUNT_TEAM", Placeholder: "1999", Help: "A plan without an amount cannot be bought here.", validate: validAmount},
				{Key: "amount_business", Label: "Amount per period: Business, UAH", Env: "WAYFORPAY_AMOUNT_BUSINESS", Placeholder: "5999", validate: validAmount},
				{Key: "callback_url", Label: "Service address", Env: "WAYFORPAY_CALLBACK_URL", Placeholder: "https://api.example.com/api/v1/billing/webhooks/wayforpay", validate: validURL},
				{Key: "period_days", Label: "Days a payment buys", Env: "WAYFORPAY_PERIOD_DAYS", Default: "30", validate: validPort},
			},
		},
		{
			ID: "paymentlink", Name: "Payment link (Payoneer, Wise, PayPal…)",
			Description: "Sell through a payment link you created at any service: a Payoneer payment request, a Wise or PayPal.me link, a bank's hosted page. There is no callback: the customer is sent to the link, the request appears under billing events, and you assign the plan with its period on the organization's page once the money arrived.",
			Fields: []Field{
				{Key: "label", Label: "Name shown to customers", Env: "PAYMENTLINK_LABEL", Placeholder: "Payoneer", validate: validUser},
				{Key: "url_team", Label: "Link: Team", Env: "PAYMENTLINK_URL_TEAM", Placeholder: "https://link.payoneer.com/…", validate: validURL},
				{Key: "url_business", Label: "Link: Business", Env: "PAYMENTLINK_URL_BUSINESS", Placeholder: "https://link.payoneer.com/…", validate: validURL},
				{Key: "url_enterprise", Label: "Link: Enterprise", Env: "PAYMENTLINK_URL_ENTERPRISE", validate: validURL},
				{Key: "instructions", Label: "What the customer sees after choosing", Env: "PAYMENTLINK_INSTRUCTIONS", Placeholder: "Pay with the link; write your organization name in the comment. The plan is activated within one business day.", validate: validNote},
			},
		},
		{
			ID: "invoice", Name: "Invoice (bank transfer)",
			Description: "For companies that pay by bank transfer. No payment page: a customer asks for a plan, you send the invoice and, once it is paid, assign the plan with its period on the organization's page.",
			Fields: []Field{
				{Key: "contact", Label: "Billing contact", Required: true, Env: "BILLING_CONTACT", Placeholder: "billing@example.com", Help: "Shown to customers asking for an invoice.", validate: validContact},
			},
		},
		{
			ID: "smtp", Name: "E-mail (SMTP)", Testable: true,
			Description: "Sends e-mail notifications to organization members. TLS is required.",
			Fields: []Field{
				{Key: "host", Label: "Server", Required: true, Env: "SMTP_HOST", Placeholder: "smtp.example.com", validate: validHost},
				{Key: "port", Label: "Port", Env: "SMTP_PORT", Default: "587", Placeholder: "587", Help: "587 (STARTTLS) or 465 (implicit TLS).", validate: validPort},
				{Key: "username", Label: "User name", Env: "SMTP_USERNAME", validate: validUser},
				{Key: "password", Label: "Password", Secret: true, Env: "SMTP_PASSWORD", Help: "Stored sealed, shown only as configured.", validate: validSecret(1)},
				{Key: "from", Label: "Sender address", Required: true, Env: "SMTP_FROM", Placeholder: "Reliabilix <notifications@example.com>", validate: validFrom},
			},
		},
		{
			ID: "platform_aws", Name: "Platform AWS account", Startup: true,
			Description: "The AWS account customers trust in the role they create for read-only cost access. Shown in their setup steps.",
			Fields: []Field{
				{Key: "account_id", Label: "Account id", Required: true, Env: "PLATFORM_AWS_ACCOUNT_ID", Placeholder: "123456789012", validate: match(awsAccount, "12 digits")},
			},
		},
		{
			ID: "platform_gcp", Name: "Platform Google Cloud project", Startup: true,
			Description: "The platform's own Google identity that reads customers' billing export datasets. Without a project the GCP connector is off.",
			Fields: []Field{
				{Key: "project", Label: "Job project", Env: "PLATFORM_GCP_PROJECT", Placeholder: "reliabilix-platform", Help: "The project in which BigQuery jobs run.", validate: match(gcpProject, "a Google Cloud project id")},
				{Key: "service_account", Label: "Service account", Env: "PLATFORM_GCP_SERVICE_ACCOUNT", Placeholder: "reader@reliabilix-platform.iam.gserviceaccount.com", Help: "Shown to customers so they can grant it read access to their dataset.", validate: match(gcpSA, "a service account e-mail")},
			},
		},
		{
			ID: "object_storage", Name: "Object storage (S3)", Startup: true,
			Description: "Where generated reports and the raw billing archive are kept. Any S3-compatible service.",
			Fields: []Field{
				{Key: "bucket", Label: "Bucket", Required: true, Env: "S3_BUCKET", validate: match(bucket, "a bucket name")},
				{Key: "endpoint", Label: "Endpoint", Env: "S3_ENDPOINT", Placeholder: "https://s3.example.com", Help: "Empty for AWS S3.", validate: validURL},
				{Key: "region", Label: "Region", Env: "S3_REGION", Default: "us-east-1", validate: match(region, "a region id")},
				{Key: "path_style", Label: "Path-style addressing", Env: "S3_PATH_STYLE", Default: "false", Help: "true for MinIO and Cloudflare R2.", validate: validBool},
				{Key: "access_key", Label: "Access key id", Env: "S3_ACCESS_KEY", Help: "Leave empty to use the server's own cloud identity (recommended in production).", validate: validSecret(3)},
				{Key: "secret_key", Label: "Secret access key", Secret: true, Env: "S3_SECRET_KEY", validate: validSecret(8)},
			},
		},
		{
			ID: "observability", Name: "Error tracking and tracing", Startup: true,
			Description: "Where unhandled errors and traces are sent.",
			Fields: []Field{
				{Key: "sentry_dsn", Label: "Sentry DSN", Secret: true, Env: "SENTRY_DSN", Placeholder: "https://key@o0.ingest.sentry.io/0", validate: validURL},
				{Key: "otlp_endpoint", Label: "OTLP endpoint", Env: "OTEL_EXPORTER_OTLP_ENDPOINT", Placeholder: "http://collector:4318", validate: validURL},
			},
		},
	}
}

// Lookup finds an integration and one of its fields.
func Lookup(id, key string) (Integration, Field, bool) {
	for _, in := range Catalog() {
		if in.ID != id {
			continue
		}
		if key == "" {
			return in, Field{}, true
		}
		for _, f := range in.Fields {
			if f.Key == key {
				return in, f, true
			}
		}
	}
	return Integration{}, Field{}, false
}

// Validate checks a value for a field.
func (f Field) Validate(v string) error {
	if v == "" {
		return fmt.Errorf("%w: empty", ErrField)
	}
	if f.validate != nil {
		return f.validate(v)
	}
	return nil
}
