package billing

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/md5" //nolint:gosec // WayForPay defines its signature as HMAC-MD5
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// WayForPay (Ukraine): cards, Apple Pay, Google Pay and instalments in hryvnia. A hosted invoice page is created through
// the API; the notification is signed (HMAC-MD5 over fixed fields) and must be answered with a signed "accept".
// It sells a period: one payment buys PeriodDays of the plan.
type WayForPay struct {
	Values  Values
	BaseURL string // default https://api.wayforpay.com
	HTTP    *http.Client
	Now     func() time.Time
}

func (w WayForPay) ID() string                               { return "wayforpay" }
func (w WayForPay) Name() string                             { return "WayForPay" }
func (w WayForPay) val(ctx context.Context, k string) string { return w.Values(ctx, "wayforpay", k) }
func (w WayForPay) base() string {
	if w.BaseURL != "" {
		return w.BaseURL
	}
	return "https://api.wayforpay.com"
}
func (w WayForPay) client() *http.Client {
	if w.HTTP != nil {
		return w.HTTP
	}
	return &http.Client{Timeout: 15 * time.Second}
}
func (w WayForPay) now() time.Time {
	if w.Now != nil {
		return w.Now()
	}
	return time.Now()
}

func (w WayForPay) Configured(ctx context.Context) bool {
	return w.val(ctx, "merchant_account") != "" && w.val(ctx, "secret_key") != "" && w.val(ctx, "merchant_domain") != "" && len(w.Plans(ctx)) > 0
}

func (w WayForPay) Plans(ctx context.Context) []string {
	var out []string
	for _, p := range []string{"team", "business"} {
		if w.val(ctx, "amount_"+p) != "" {
			out = append(out, p)
		}
	}
	return out
}

func (w WayForPay) periodDays(ctx context.Context) int {
	if n, err := strconv.Atoi(w.val(ctx, "period_days")); err == nil && n >= 1 && n <= 400 {
		return n
	}
	return 30
}

func hmacMD5(secret, msg string) string {
	m := hmac.New(md5.New, []byte(secret)) //nolint:gosec
	m.Write([]byte(msg))
	return hex.EncodeToString(m.Sum(nil))
}

func (w WayForPay) Checkout(ctx context.Context, req CheckoutRequest) (CheckoutResult, error) {
	account, secret, domain := w.val(ctx, "merchant_account"), w.val(ctx, "secret_key"), w.val(ctx, "merchant_domain")
	if account == "" || secret == "" || domain == "" {
		return CheckoutResult{}, ErrNotConfigured
	}
	amount := w.val(ctx, "amount_"+req.Plan)
	if f, err := strconv.ParseFloat(amount, 64); err != nil || f <= 0 {
		return CheckoutResult{}, ErrUnknownPlan
	}
	rb := make([]byte, 6)
	_, _ = rand.Read(rb)
	ref := req.OrganizationID + ":" + req.Plan + ":" + hex.EncodeToString(rb)
	date := strconv.FormatInt(w.now().Unix(), 10)
	product := "Reliabilix GreenOps: " + req.Plan
	sig := hmacMD5(secret, strings.Join([]string{account, domain, ref, date, amount, "UAH", product, "1", amount}, ";"))
	payload := map[string]any{
		"transactionType": "CREATE_INVOICE", "merchantAccount": account, "merchantAuthType": "SimpleSignature", "merchantDomainName": domain,
		"merchantSignature": sig, "apiVersion": 1, "language": "UA", "orderReference": ref, "orderDate": date, "amount": amount, "currency": "UAH",
		"productName": []string{product}, "productPrice": []string{amount}, "productCount": []int{1}, "returnUrl": req.ReturnURL,
	}
	if cb := w.val(ctx, "callback_url"); cb != "" {
		payload["serviceUrl"] = cb
	}
	b, _ := json.Marshal(payload)
	r, err := http.NewRequestWithContext(ctx, http.MethodPost, w.base()+"/api", bytes.NewReader(b))
	if err != nil {
		return CheckoutResult{}, err
	}
	r.Header.Set("Content-Type", "application/json")
	resp, err := w.client().Do(r)
	if err != nil {
		return CheckoutResult{}, fmt.Errorf("wayforpay: request failed")
	}
	defer func() { _ = resp.Body.Close() }()
	var out struct {
		Reason     string `json:"reason"`
		ReasonCode int    `json:"reasonCode"`
		InvoiceURL string `json:"invoiceUrl"`
	}
	if resp.StatusCode != http.StatusOK || json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&out) != nil || out.InvoiceURL == "" {
		return CheckoutResult{}, fmt.Errorf("wayforpay: invoice was refused (status %d, code %d)", resp.StatusCode, out.ReasonCode)
	}
	return CheckoutResult{URL: out.InvoiceURL}, nil
}

type wfpNotice struct {
	MerchantAccount   string `json:"merchantAccount"`
	OrderReference    string `json:"orderReference"`
	Amount            any    `json:"amount"`
	Currency          string `json:"currency"`
	AuthCode          string `json:"authCode"`
	CardPan           string `json:"cardPan"`
	TransactionStatus string `json:"transactionStatus"`
	ReasonCode        any    `json:"reasonCode"`
	MerchantSignature string `json:"merchantSignature"`
}

// field renders a JSON number or string the way WayForPay signs it.
func field(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case float64:
		return strconv.FormatFloat(x, 'f', -1, 64)
	case nil:
		return ""
	}
	return fmt.Sprint(v)
}

func (w WayForPay) ParseWebhook(ctx context.Context, _ *http.Request, body []byte) (*Event, error) {
	secret := w.val(ctx, "secret_key")
	if secret == "" {
		return nil, ErrNotConfigured
	}
	var n wfpNotice
	if err := json.Unmarshal(body, &n); err != nil || n.OrderReference == "" {
		return nil, ErrBadSignature
	}
	want := hmacMD5(secret, strings.Join([]string{n.MerchantAccount, n.OrderReference, field(n.Amount), n.Currency, n.AuthCode, n.CardPan, n.TransactionStatus, field(n.ReasonCode)}, ";"))
	if !hmac.Equal([]byte(want), []byte(strings.ToLower(n.MerchantSignature))) || n.MerchantAccount != w.val(ctx, "merchant_account") {
		return nil, ErrBadSignature
	}
	parts := strings.Split(n.OrderReference, ":")
	if len(parts) < 2 {
		return nil, nil
	}
	e := &Event{ID: n.OrderReference + ":" + n.TransactionStatus, OrganizationID: parts[0], Plan: parts[1], Reference: n.OrderReference}
	switch n.TransactionStatus {
	case "Approved":
		end := w.now().UTC().AddDate(0, 0, w.periodDays(ctx))
		e.Status, e.PeriodEnd = "active", &end
	case "Refunded", "Voided":
		e.Status, e.Plan = "canceled", "free"
	default:
		return nil, nil // Pending, InProcessing, Declined, Expired ...: nothing changes
	}
	return e, nil
}

// Ack is the answer WayForPay requires for every notification: a signed "accept" for the order reference.
func (w WayForPay) Ack(ctx context.Context, body []byte) any {
	var n wfpNotice
	_ = json.Unmarshal(body, &n)
	t := strconv.FormatInt(w.now().Unix(), 10)
	return map[string]any{"orderReference": n.OrderReference, "status": "accept", "time": w.now().Unix(),
		"signature": hmacMD5(w.val(ctx, "secret_key"), strings.Join([]string{n.OrderReference, "accept", t}, ";"))}
}

// Price implements Pricer: the configured amount for the plan, in hryvnia.
func (w WayForPay) Price(ctx context.Context, plan string) (float64, string, bool) {
	v, err := strconv.ParseFloat(w.val(ctx, "amount_"+plan), 64)
	if err != nil || v <= 0 {
		return 0, "", false
	}
	return v, "UAH", true
}
