package billing

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// Stripe: hosted Checkout in subscription mode; subscription lifecycle through webhooks.
type Stripe struct {
	Values  Values
	BaseURL string // default https://api.stripe.com
	HTTP    *http.Client
	Now     func() time.Time
}

func (s Stripe) ID() string                               { return "stripe" }
func (s Stripe) Name() string                             { return "Stripe" }
func (s Stripe) val(ctx context.Context, k string) string { return s.Values(ctx, "stripe", k) }
func (s Stripe) base() string {
	if s.BaseURL != "" {
		return s.BaseURL
	}
	return "https://api.stripe.com"
}
func (s Stripe) client() *http.Client {
	if s.HTTP != nil {
		return s.HTTP
	}
	return &http.Client{Timeout: 15 * time.Second}
}
func (s Stripe) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now()
}

func (s Stripe) priceFor(ctx context.Context, plan string) string { return s.val(ctx, "price_"+plan) }

func (s Stripe) Configured(ctx context.Context) bool {
	return s.val(ctx, "secret_key") != "" && s.val(ctx, "webhook_secret") != "" && len(s.Plans(ctx)) > 0
}

func (s Stripe) Plans(ctx context.Context) []string {
	var out []string
	for _, p := range []string{"team", "business"} {
		if s.priceFor(ctx, p) != "" {
			out = append(out, p)
		}
	}
	return out
}

func (s Stripe) Checkout(ctx context.Context, req CheckoutRequest) (CheckoutResult, error) {
	key, price := s.val(ctx, "secret_key"), s.priceFor(ctx, req.Plan)
	if key == "" {
		return CheckoutResult{}, ErrNotConfigured
	}
	if price == "" {
		return CheckoutResult{}, ErrUnknownPlan
	}
	f := url.Values{}
	f.Set("mode", "subscription")
	f.Set("line_items[0][price]", price)
	f.Set("line_items[0][quantity]", "1")
	f.Set("success_url", req.ReturnURL)
	f.Set("cancel_url", req.ReturnURL)
	f.Set("client_reference_id", req.OrganizationID)
	if req.Email != "" {
		f.Set("customer_email", req.Email)
	}
	for _, p := range []string{"metadata", "subscription_data[metadata]"} {
		f.Set(p+"[organization_id]", req.OrganizationID)
		f.Set(p+"[plan]", req.Plan)
	}
	r, err := http.NewRequestWithContext(ctx, http.MethodPost, s.base()+"/v1/checkout/sessions", strings.NewReader(f.Encode()))
	if err != nil {
		return CheckoutResult{}, err
	}
	r.Header.Set("Authorization", "Bearer "+key)
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := s.client().Do(r)
	if err != nil {
		return CheckoutResult{}, fmt.Errorf("stripe: request failed")
	}
	defer func() { _ = resp.Body.Close() }()
	var out struct {
		URL string `json:"url"`
	}
	if resp.StatusCode != http.StatusOK || json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&out) != nil || out.URL == "" {
		return CheckoutResult{}, fmt.Errorf("stripe: checkout was refused (status %d)", resp.StatusCode)
	}
	return CheckoutResult{URL: out.URL}, nil
}

// verify checks the Stripe-Signature header: t=<unix>,v1=<hex hmac-sha256 of "t.payload">, within five minutes.
func (s Stripe) verify(ctx context.Context, header string, body []byte) error {
	secret := s.val(ctx, "webhook_secret")
	if secret == "" {
		return ErrNotConfigured
	}
	var ts string
	var sigs []string
	for _, part := range strings.Split(header, ",") {
		k, v, _ := strings.Cut(strings.TrimSpace(part), "=")
		switch k {
		case "t":
			ts = v
		case "v1":
			sigs = append(sigs, v)
		}
	}
	t, err := strconv.ParseInt(ts, 10, 64)
	if err != nil || len(sigs) == 0 {
		return ErrBadSignature
	}
	if d := s.now().Sub(time.Unix(t, 0)); d > 5*time.Minute || d < -5*time.Minute {
		return ErrBadSignature
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(ts + "."))
	mac.Write(body)
	want := hex.EncodeToString(mac.Sum(nil))
	for _, sig := range sigs {
		if hmac.Equal([]byte(sig), []byte(want)) {
			return nil
		}
	}
	return ErrBadSignature
}

func (s Stripe) ParseWebhook(ctx context.Context, r *http.Request, body []byte) (*Event, error) {
	if err := s.verify(ctx, r.Header.Get("Stripe-Signature"), body); err != nil {
		return nil, err
	}
	var ev struct {
		ID   string `json:"id"`
		Type string `json:"type"`
		Data struct {
			Object struct {
				ID               string            `json:"id"`
				Status           string            `json:"status"`
				CurrentPeriodEnd int64             `json:"current_period_end"`
				Metadata         map[string]string `json:"metadata"`
			} `json:"object"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &ev); err != nil {
		return nil, ErrBadSignature
	}
	switch ev.Type {
	case "customer.subscription.created", "customer.subscription.updated", "customer.subscription.deleted":
	default:
		return nil, nil
	}
	o := ev.Data.Object
	org, plan := o.Metadata["organization_id"], o.Metadata["plan"]
	if org == "" || plan == "" {
		return nil, nil // a subscription that did not start from this platform
	}
	status := map[string]string{"active": "active", "trialing": "trialing", "past_due": "past_due", "canceled": "canceled", "unpaid": "canceled", "incomplete_expired": "canceled"}[o.Status]
	if ev.Type == "customer.subscription.deleted" {
		status = "canceled"
	}
	if status == "" {
		return nil, nil // incomplete, paused: not a state the platform acts on
	}
	e := &Event{ID: ev.ID, OrganizationID: org, Plan: plan, Status: status, Reference: o.ID}
	if o.CurrentPeriodEnd > 0 {
		t := time.Unix(o.CurrentPeriodEnd, 0).UTC()
		e.PeriodEnd = &t
	}
	if status == "canceled" {
		e.Plan = "free"
	}
	return e, nil
}
