package billing

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// Paddle (Billing): a merchant of record, so it also handles sales tax and VAT for the customer's country.
type Paddle struct {
	Values  Values
	BaseURL string // default https://api.paddle.com (https://sandbox-api.paddle.com when "sandbox" is true)
	HTTP    *http.Client
	Now     func() time.Time
}

func (p Paddle) ID() string                               { return "paddle" }
func (p Paddle) Name() string                             { return "Paddle" }
func (p Paddle) val(ctx context.Context, k string) string { return p.Values(ctx, "paddle", k) }
func (p Paddle) base(ctx context.Context) string {
	switch {
	case p.BaseURL != "":
		return p.BaseURL
	case p.val(ctx, "sandbox") == "true":
		return "https://sandbox-api.paddle.com"
	}
	return "https://api.paddle.com"
}
func (p Paddle) client() *http.Client {
	if p.HTTP != nil {
		return p.HTTP
	}
	return &http.Client{Timeout: 15 * time.Second}
}
func (p Paddle) now() time.Time {
	if p.Now != nil {
		return p.Now()
	}
	return time.Now()
}

func (p Paddle) Configured(ctx context.Context) bool {
	return p.val(ctx, "api_key") != "" && p.val(ctx, "webhook_secret") != "" && len(p.Plans(ctx)) > 0
}

func (p Paddle) Plans(ctx context.Context) []string {
	var out []string
	for _, pl := range []string{"team", "business"} {
		if p.val(ctx, "price_"+pl) != "" {
			out = append(out, pl)
		}
	}
	return out
}

func (p Paddle) Checkout(ctx context.Context, req CheckoutRequest) (CheckoutResult, error) {
	key, price := p.val(ctx, "api_key"), p.val(ctx, "price_"+req.Plan)
	if key == "" {
		return CheckoutResult{}, ErrNotConfigured
	}
	if price == "" {
		return CheckoutResult{}, ErrUnknownPlan
	}
	body, _ := json.Marshal(map[string]any{
		"items":       []map[string]any{{"price_id": price, "quantity": 1}},
		"custom_data": map[string]string{"organization_id": req.OrganizationID, "plan": req.Plan},
	})
	r, err := http.NewRequestWithContext(ctx, http.MethodPost, p.base(ctx)+"/transactions", bytes.NewReader(body))
	if err != nil {
		return CheckoutResult{}, err
	}
	r.Header.Set("Authorization", "Bearer "+key)
	r.Header.Set("Content-Type", "application/json")
	resp, err := p.client().Do(r)
	if err != nil {
		return CheckoutResult{}, fmt.Errorf("paddle: request failed")
	}
	defer func() { _ = resp.Body.Close() }()
	var out struct {
		Data struct {
			Checkout struct {
				URL string `json:"url"`
			} `json:"checkout"`
		} `json:"data"`
	}
	if (resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated) || json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&out) != nil || out.Data.Checkout.URL == "" {
		return CheckoutResult{}, fmt.Errorf("paddle: checkout was refused (status %d)", resp.StatusCode)
	}
	return CheckoutResult{URL: out.Data.Checkout.URL}, nil
}

// verify checks Paddle-Signature: ts=<unix>;h1=<hex hmac-sha256 of "ts:payload">, within five minutes.
func (p Paddle) verify(ctx context.Context, header string, body []byte) error {
	secret := p.val(ctx, "webhook_secret")
	if secret == "" {
		return ErrNotConfigured
	}
	var ts string
	var sigs []string
	for _, part := range strings.Split(header, ";") {
		k, v, _ := strings.Cut(strings.TrimSpace(part), "=")
		switch k {
		case "ts":
			ts = v
		case "h1":
			sigs = append(sigs, v)
		}
	}
	t, err := strconv.ParseInt(ts, 10, 64)
	if err != nil || len(sigs) == 0 {
		return ErrBadSignature
	}
	if d := p.now().Sub(time.Unix(t, 0)); d > 5*time.Minute || d < -5*time.Minute {
		return ErrBadSignature
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(ts + ":"))
	mac.Write(body)
	want := hex.EncodeToString(mac.Sum(nil))
	for _, s := range sigs {
		if hmac.Equal([]byte(s), []byte(want)) {
			return nil
		}
	}
	return ErrBadSignature
}

func (p Paddle) ParseWebhook(ctx context.Context, r *http.Request, body []byte) (*Event, error) {
	if err := p.verify(ctx, r.Header.Get("Paddle-Signature"), body); err != nil {
		return nil, err
	}
	var ev struct {
		EventID   string `json:"event_id"`
		EventType string `json:"event_type"`
		Data      struct {
			ID                   string            `json:"id"`
			Status               string            `json:"status"`
			CustomData           map[string]string `json:"custom_data"`
			CurrentBillingPeriod struct {
				EndsAt time.Time `json:"ends_at"`
			} `json:"current_billing_period"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &ev); err != nil {
		return nil, ErrBadSignature
	}
	if !strings.HasPrefix(ev.EventType, "subscription.") {
		return nil, nil
	}
	org, plan := ev.Data.CustomData["organization_id"], ev.Data.CustomData["plan"]
	if org == "" || plan == "" {
		return nil, nil
	}
	status := map[string]string{"active": "active", "trialing": "trialing", "past_due": "past_due", "canceled": "canceled"}[ev.Data.Status]
	if status == "" {
		return nil, nil // paused and unknown states are not acted on
	}
	e := &Event{ID: ev.EventID, OrganizationID: org, Plan: plan, Status: status, Reference: ev.Data.ID}
	if !ev.Data.CurrentBillingPeriod.EndsAt.IsZero() {
		t := ev.Data.CurrentBillingPeriod.EndsAt.UTC()
		e.PeriodEnd = &t
	}
	if status == "canceled" {
		e.Plan = "free"
	}
	return e, nil
}
