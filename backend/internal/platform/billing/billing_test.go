package billing

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

var fixedNow = time.Date(2026, 10, 10, 12, 0, 0, 0, time.UTC)

func vals(m map[string]string) Values {
	return func(_ context.Context, integration, key string) string { return m[integration+"."+key] }
}

func hmacHex(secret, msg string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(msg))
	return hex.EncodeToString(mac.Sum(nil))
}

func hook(headerName, headerValue string) *http.Request {
	r := httptest.NewRequest("POST", "/hook", nil)
	r.Header.Set(headerName, headerValue)
	return r
}

func TestStripeWebhookIsVerifiedAndReduced(t *testing.T) {
	s := Stripe{Values: vals(map[string]string{"stripe.webhook_secret": "whsec", "stripe.secret_key": "sk", "stripe.price_team": "price_1"}), Now: func() time.Time { return fixedNow }}
	body := []byte(`{"id":"evt_1","type":"customer.subscription.updated","data":{"object":{"id":"sub_1","status":"active","current_period_end":1790000000,"metadata":{"organization_id":"11111111-1111-1111-1111-111111111111","plan":"team"}}}}`)
	ts := fmt.Sprint(fixedNow.Unix())
	good := fmt.Sprintf("t=%s,v1=%s", ts, hmacHex("whsec", ts+"."+string(body)))
	e, err := s.ParseWebhook(context.Background(), hook("Stripe-Signature", good), body)
	if err != nil || e == nil || e.ID != "evt_1" || e.Plan != "team" || e.Status != "active" || e.PeriodEnd == nil || e.PeriodEnd.Unix() != 1790000000 {
		t.Fatalf("%+v %v", e, err)
	}
	// forged, replayed (old timestamp) and altered bodies are refused
	for name, h := range map[string]string{
		"forged":   fmt.Sprintf("t=%s,v1=%s", ts, hmacHex("other", ts+"."+string(body))),
		"stale":    fmt.Sprintf("t=%d,v1=%s", fixedNow.Add(-time.Hour).Unix(), hmacHex("whsec", fmt.Sprint(fixedNow.Add(-time.Hour).Unix())+"."+string(body))),
		"no value": "",
	} {
		if _, err := s.ParseWebhook(context.Background(), hook("Stripe-Signature", h), body); !errors.Is(err, ErrBadSignature) {
			t.Errorf("%s = %v, want ErrBadSignature", name, err)
		}
	}
	if _, err := s.ParseWebhook(context.Background(), hook("Stripe-Signature", good), append([]byte(nil), body[:len(body)-1]...)); !errors.Is(err, ErrBadSignature) {
		t.Errorf("an altered body must be refused: %v", err)
	}
	// a cancelled subscription returns the organization to the free plan
	del := []byte(`{"id":"evt_2","type":"customer.subscription.deleted","data":{"object":{"id":"sub_1","status":"canceled","metadata":{"organization_id":"11111111-1111-1111-1111-111111111111","plan":"team"}}}}`)
	e, err = s.ParseWebhook(context.Background(), hook("Stripe-Signature", fmt.Sprintf("t=%s,v1=%s", ts, hmacHex("whsec", ts+"."+string(del)))), del)
	if err != nil || e == nil || e.Plan != "free" || e.Status != "canceled" {
		t.Errorf("%+v %v", e, err)
	}
	// events of other kinds, and subscriptions that did not start here, are ignored
	other := []byte(`{"id":"evt_3","type":"invoice.paid","data":{"object":{}}}`)
	if e, err := s.ParseWebhook(context.Background(), hook("Stripe-Signature", fmt.Sprintf("t=%s,v1=%s", ts, hmacHex("whsec", ts+"."+string(other)))), other); e != nil || err != nil {
		t.Errorf("other event = %+v %v", e, err)
	}
}

func TestStripeCheckoutSendsThePriceAndTheOrganization(t *testing.T) {
	var got string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		got = r.Header.Get("Authorization") + "|" + string(b)
		_, _ = w.Write([]byte(`{"url":"https://checkout.stripe.test/c/1"}`))
	}))
	defer srv.Close()
	s := Stripe{BaseURL: srv.URL, Values: vals(map[string]string{"stripe.secret_key": "sk_x", "stripe.webhook_secret": "w", "stripe.price_team": "price_T"})}
	res, err := s.Checkout(context.Background(), CheckoutRequest{OrganizationID: "org-1", Email: "a@b.co", Plan: "team", ReturnURL: "https://app/settings"})
	if err != nil || res.URL != "https://checkout.stripe.test/c/1" {
		t.Fatalf("%+v %v", res, err)
	}
	for _, want := range []string{"Bearer sk_x", "price_T", "client_reference_id=org-1", "subscription_data%5Bmetadata%5D%5Borganization_id%5D=org-1", "mode=subscription"} {
		if !strings.Contains(got, want) {
			t.Errorf("request lacks %q: %s", want, got)
		}
	}
	if _, err := s.Checkout(context.Background(), CheckoutRequest{Plan: "business"}); !errors.Is(err, ErrUnknownPlan) {
		t.Errorf("a plan without a price = %v", err)
	}
}

func TestPaddleWebhook(t *testing.T) {
	p := Paddle{Values: vals(map[string]string{"paddle.webhook_secret": "pdl", "paddle.api_key": "k", "paddle.price_team": "pri_1"}), Now: func() time.Time { return fixedNow }}
	body := []byte(`{"event_id":"evt_p1","event_type":"subscription.updated","data":{"id":"sub_p","status":"past_due","custom_data":{"organization_id":"22222222-2222-2222-2222-222222222222","plan":"business"},"current_billing_period":{"ends_at":"2026-11-10T00:00:00Z"}}}`)
	ts := fmt.Sprint(fixedNow.Unix())
	e, err := p.ParseWebhook(context.Background(), hook("Paddle-Signature", "ts="+ts+";h1="+hmacHex("pdl", ts+":"+string(body))), body)
	if err != nil || e == nil || e.Status != "past_due" || e.Plan != "business" || e.PeriodEnd == nil || e.PeriodEnd.Month() != 11 {
		t.Fatalf("%+v %v", e, err)
	}
	if _, err := p.ParseWebhook(context.Background(), hook("Paddle-Signature", "ts="+ts+";h1=00"), body); !errors.Is(err, ErrBadSignature) {
		t.Errorf("forged = %v", err)
	}
}

func TestLiqPaySignsTheCheckoutAndVerifiesTheCallback(t *testing.T) {
	l := LiqPay{Now: func() time.Time { return fixedNow }, Values: vals(map[string]string{"liqpay.public_key": "pub", "liqpay.private_key": "priv", "liqpay.amount_team": "1999", "liqpay.callback_url": "https://api.example/cb", "liqpay.paytypes": "card,apay,gpay"})}
	res, err := l.Checkout(context.Background(), CheckoutRequest{OrganizationID: "44444444-4444-4444-4444-444444444444", Plan: "team", ReturnURL: "https://app/settings"})
	if err != nil || !strings.HasPrefix(res.URL, "https://www.liqpay.ua/api/3/checkout?") {
		t.Fatalf("%+v %v", res, err)
	}
	u, _ := url.Parse(res.URL)
	data := u.Query().Get("data")
	if u.Query().Get("signature") != sign("priv", data) {
		t.Error("the checkout request must be signed with the private key")
	}
	raw, _ := base64.StdEncoding.DecodeString(data)
	for _, want := range []string{`"action":"subscribe"`, `"currency":"UAH"`, `"amount":1999`, "44444444-4444-4444-4444-444444444444:team:", `"server_url":"https://api.example/cb"`, `"paytypes":"card,apay,gpay"`} {
		if !strings.Contains(string(raw), want) {
			t.Errorf("payload lacks %s: %s", want, raw)
		}
	}
	if strings.Contains(res.URL, "priv") {
		t.Error("the private key must never appear in the link")
	}

	cb := func(status string) []byte {
		d := base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf(`{"status":%q,"order_id":"44444444-4444-4444-4444-444444444444:team:abc","payment_id":777}`, status)))
		return []byte(url.Values{"data": {d}, "signature": {sign("priv", d)}}.Encode())
	}
	e, err := l.ParseWebhook(context.Background(), httptest.NewRequest("POST", "/h", nil), cb("success"))
	if err != nil || e == nil || e.Status != "active" || e.Plan != "team" || e.OrganizationID != "44444444-4444-4444-4444-444444444444" || !e.PeriodEnd.Equal(fixedNow.AddDate(0, 0, 30)) {
		t.Fatalf("%+v %v", e, err)
	}
	if e, _ := l.ParseWebhook(context.Background(), httptest.NewRequest("POST", "/h", nil), cb("unsubscribed")); e == nil || e.Plan != "free" || e.Status != "canceled" {
		t.Errorf("unsubscribe = %+v", e)
	}
	if e, err := l.ParseWebhook(context.Background(), httptest.NewRequest("POST", "/h", nil), cb("failure")); e != nil || err != nil {
		t.Errorf("a failed payment must not change the plan: %+v %v", e, err)
	}
	forged := url.Values{"data": {base64.StdEncoding.EncodeToString([]byte(`{"status":"success","order_id":"x:team:1"}`))}, "signature": {"AAAA"}}.Encode()
	if _, err := l.ParseWebhook(context.Background(), httptest.NewRequest("POST", "/h", nil), []byte(forged)); !errors.Is(err, ErrBadSignature) {
		t.Errorf("forged = %v", err)
	}
}

func TestWayForPayChecksTheSignatureAndAnswersWithASignedAccept(t *testing.T) {
	w := WayForPay{Now: func() time.Time { return fixedNow }, Values: vals(map[string]string{
		"wayforpay.merchant_account": "acme", "wayforpay.secret_key": "wfp-secret", "wayforpay.merchant_domain": "app.example.com", "wayforpay.amount_team": "1999"})}
	notice := func(status, sig string) []byte {
		if sig == "" {
			sig = hmacMD5("wfp-secret", strings.Join([]string{"acme", "55555555-5555-5555-5555-555555555555:team:ab12", "1999", "UAH", "A1", "41****1111", status, "1100"}, ";"))
		}
		return []byte(fmt.Sprintf(`{"merchantAccount":"acme","orderReference":"55555555-5555-5555-5555-555555555555:team:ab12","amount":1999,"currency":"UAH","authCode":"A1","cardPan":"41****1111","transactionStatus":%q,"reasonCode":1100,"merchantSignature":%q}`, status, sig))
	}
	e, err := w.ParseWebhook(context.Background(), httptest.NewRequest("POST", "/h", nil), notice("Approved", ""))
	if err != nil || e == nil || e.Status != "active" || e.Plan != "team" || !e.PeriodEnd.Equal(fixedNow.AddDate(0, 0, 30)) {
		t.Fatalf("%+v %v", e, err)
	}
	if e, _ := w.ParseWebhook(context.Background(), httptest.NewRequest("POST", "/h", nil), notice("Refunded", "")); e == nil || e.Plan != "free" || e.Status != "canceled" {
		t.Errorf("refund = %+v", e)
	}
	if e, err := w.ParseWebhook(context.Background(), httptest.NewRequest("POST", "/h", nil), notice("Declined", "")); e != nil || err != nil {
		t.Errorf("a declined payment must not change the plan: %+v %v", e, err)
	}
	if _, err := w.ParseWebhook(context.Background(), httptest.NewRequest("POST", "/h", nil), notice("Approved", "deadbeef")); !errors.Is(err, ErrBadSignature) {
		t.Errorf("forged = %v", err)
	}
	ack, _ := w.Ack(context.Background(), notice("Approved", "")).(map[string]any)
	ts := fmt.Sprint(fixedNow.Unix())
	if ack["status"] != "accept" || ack["signature"] != hmacMD5("wfp-secret", "55555555-5555-5555-5555-555555555555:team:ab12;accept;"+ts) {
		t.Errorf("ack = %v", ack)
	}
}

func TestPaymentLinkSendsThePersonToTheLinkAndIsConfirmedByHand(t *testing.T) {
	p := PaymentLink{Values: vals(map[string]string{"paymentlink.label": "Payoneer", "paymentlink.url_team": "https://link.payoneer.com/team"})}
	if !p.Configured(context.Background()) || !p.Manual() {
		t.Fatal("a link makes it usable, and a payment is confirmed by hand")
	}
	if got := p.Plans(context.Background()); len(got) != 1 || got[0] != "team" {
		t.Errorf("plans = %v", got)
	}
	res, err := p.Checkout(context.Background(), CheckoutRequest{Plan: "team"})
	if err != nil || res.URL != "https://link.payoneer.com/team" || res.Message == "" {
		t.Errorf("%+v %v", res, err)
	}
	if _, err := p.Checkout(context.Background(), CheckoutRequest{Plan: "business"}); !errors.Is(err, ErrUnknownPlan) {
		t.Errorf("a plan without a link = %v", err)
	}
	if e, err := p.ParseWebhook(context.Background(), nil, nil); e != nil || err != nil {
		t.Error("there is no callback to parse")
	}
}

func TestRegistryAndInvoice(t *testing.T) {
	v := vals(map[string]string{"invoice.contact": "billing@example.com"})
	r := NewRegistry(Stripe{Values: v}, Invoice{Values: v}, Paddle{Values: v}, LiqPay{Values: v}, WayForPay{Values: v}, PaymentLink{Values: v})
	ids := []string{}
	for _, p := range r.All() {
		ids = append(ids, p.ID())
	}
	if strings.Join(ids, ",") != "invoice,liqpay,paddle,paymentlink,stripe,wayforpay" {
		t.Errorf("providers = %v", ids)
	}
	inv, _ := r.Get("invoice")
	if !inv.Configured(context.Background()) {
		t.Error("invoice with a contact must be configured")
	}
	res, err := inv.Checkout(context.Background(), CheckoutRequest{Plan: "team", Email: "x@y.z"})
	if err != nil || res.URL != "" || !strings.Contains(res.Message, "x@y.z") {
		t.Errorf("%+v %v", res, err)
	}
	if s, _ := r.Get("stripe"); s.Configured(context.Background()) {
		t.Error("a provider without credentials must not be configured")
	}
}
