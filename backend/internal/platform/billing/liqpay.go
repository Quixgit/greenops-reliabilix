package billing

import (
	"context"
	"crypto/rand"
	"crypto/sha1" //nolint:gosec // LiqPay defines its signature as base64(sha1(private_key + data + private_key))
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// LiqPay (PrivatBank, Ukraine): cards, Apple Pay, Google Pay and Privat24 payments in hryvnia, with recurring
// subscriptions. The platform sends the person to the hosted checkout with a signed request; the callback is signed the
// same way and carries the organization and the plan in order_id.
type LiqPay struct {
	Values  Values
	BaseURL string // default https://www.liqpay.ua
	Now     func() time.Time
}

func (l LiqPay) ID() string                               { return "liqpay" }
func (l LiqPay) Name() string                             { return "LiqPay" }
func (l LiqPay) val(ctx context.Context, k string) string { return l.Values(ctx, "liqpay", k) }
func (l LiqPay) base() string {
	if l.BaseURL != "" {
		return l.BaseURL
	}
	return "https://www.liqpay.ua"
}
func (l LiqPay) now() time.Time {
	if l.Now != nil {
		return l.Now()
	}
	return time.Now()
}

func (l LiqPay) Configured(ctx context.Context) bool {
	return l.val(ctx, "public_key") != "" && l.val(ctx, "private_key") != "" && len(l.Plans(ctx)) > 0
}

func (l LiqPay) Plans(ctx context.Context) []string {
	var out []string
	for _, p := range []string{"team", "business"} {
		if l.val(ctx, "amount_"+p) != "" {
			out = append(out, p)
		}
	}
	return out
}

func (l LiqPay) periodDays(ctx context.Context) int {
	if n, err := strconv.Atoi(l.val(ctx, "period_days")); err == nil && n >= 1 && n <= 400 {
		return n
	}
	return 30
}

// sign is LiqPay's signature: base64(sha1(private_key + data + private_key)).
func sign(private, data string) string {
	h := sha1.Sum([]byte(private + data + private)) //nolint:gosec
	return base64.StdEncoding.EncodeToString(h[:])
}

// Checkout builds the signed hosted-checkout link: a monthly subscription for the plan's amount in hryvnia.
func (l LiqPay) Checkout(ctx context.Context, req CheckoutRequest) (CheckoutResult, error) {
	pub, priv := l.val(ctx, "public_key"), l.val(ctx, "private_key")
	if pub == "" || priv == "" {
		return CheckoutResult{}, ErrNotConfigured
	}
	amount := l.val(ctx, "amount_"+req.Plan)
	if amount == "" {
		return CheckoutResult{}, ErrUnknownPlan
	}
	f, err := strconv.ParseFloat(amount, 64)
	if err != nil || f <= 0 {
		return CheckoutResult{}, ErrUnknownPlan
	}
	payload := map[string]any{
		"version": 3, "public_key": pub, "action": "subscribe", "amount": f, "currency": "UAH",
		"description":           fmt.Sprintf("Reliabilix GreenOps: %s plan", req.Plan),
		"order_id":              req.OrganizationID + ":" + req.Plan + ":" + idempotenceKey()[:12],
		"subscribe":             1,
		"subscribe_date_start":  l.now().UTC().Format("2006-01-02 15:04:05"),
		"subscribe_periodicity": "month",
		"result_url":            req.ReturnURL,
		"language":              "uk",
	}
	if cb := l.val(ctx, "callback_url"); cb != "" {
		payload["server_url"] = cb
	}
	// Payment methods on the checkout page; Apple Pay and Google Pay appear next to the card when the device supports them.
	if pt := l.val(ctx, "paytypes"); pt != "" {
		payload["paytypes"] = pt
	}
	raw, _ := json.Marshal(payload)
	data := base64.StdEncoding.EncodeToString(raw)
	q := url.Values{"data": {data}, "signature": {sign(priv, data)}}
	return CheckoutResult{URL: l.base() + "/api/3/checkout?" + q.Encode()}, nil
}

func idempotenceKey() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%x", b)
}

// ParseWebhook reads LiqPay's callback: form fields data and signature.
func (l LiqPay) ParseWebhook(ctx context.Context, r *http.Request, body []byte) (*Event, error) {
	priv := l.val(ctx, "private_key")
	if priv == "" {
		return nil, ErrNotConfigured
	}
	form, err := url.ParseQuery(string(body))
	if err != nil {
		return nil, ErrBadSignature
	}
	data, sig := form.Get("data"), form.Get("signature")
	if data == "" || subtle.ConstantTimeCompare([]byte(sig), []byte(sign(priv, data))) != 1 {
		return nil, ErrBadSignature
	}
	raw, err := base64.StdEncoding.DecodeString(data)
	if err != nil {
		return nil, ErrBadSignature
	}
	var p struct {
		Status    string `json:"status"`
		OrderID   string `json:"order_id"`
		PaymentID int64  `json:"payment_id"`
		ErrCode   string `json:"err_code"`
	}
	if err := json.Unmarshal(raw, &p); err != nil {
		return nil, ErrBadSignature
	}
	parts := strings.Split(p.OrderID, ":")
	if len(parts) < 2 {
		return nil, nil // not a payment that started here
	}
	e := &Event{ID: fmt.Sprintf("%d:%s", p.PaymentID, p.Status), OrganizationID: parts[0], Plan: parts[1], Reference: strconv.FormatInt(p.PaymentID, 10)}
	switch p.Status {
	case "success", "subscribed":
		end := l.now().UTC().AddDate(0, 0, l.periodDays(ctx))
		e.Status, e.PeriodEnd = "active", &end
	case "unsubscribed":
		e.Status, e.Plan = "canceled", "free"
	default:
		return nil, nil // sandbox, wait, failure and the rest do not change the plan
	}
	return e, nil
}

// Price implements Pricer: the configured amount for the plan, in hryvnia.
func (l LiqPay) Price(ctx context.Context, plan string) (float64, string, bool) {
	v, err := strconv.ParseFloat(l.val(ctx, "amount_"+plan), 64)
	if err != nil || v <= 0 {
		return 0, "", false
	}
	return v, "UAH", true
}
