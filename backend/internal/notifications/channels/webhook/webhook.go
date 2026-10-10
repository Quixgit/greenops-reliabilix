// Package webhook delivers events as signed JSON POSTs to an HTTPS endpoint the tenant controls.
//
// Request:
//
//	POST <endpoint>
//	Content-Type: application/json
//	User-Agent: Reliabilix-Notifications/1
//	X-Reliabilix-Event: report.ready
//	X-Reliabilix-Delivery: <delivery id; stable across retries, use it to deduplicate>
//	X-Reliabilix-Signature: t=<unix seconds>,v1=<hex HMAC-SHA256(secret, "<t>.<body>")>
//
// The receiver recomputes the HMAC with its signing secret and rejects stale timestamps (replay protection).
package webhook

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/quixgit/greenops-reliabilix/backend/internal/notifications/channels/safehttp"
	"github.com/quixgit/greenops-reliabilix/backend/internal/notifications/domain"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/notify"
)

// PayloadVersion is bumped only for incompatible payload changes.
const PayloadVersion = 1

type Channel struct {
	Opts   safehttp.Options
	Client *http.Client
}

func New(o safehttp.Options) *Channel { return &Channel{Opts: o, Client: safehttp.NewClient(o)} }

func (*Channel) Kind() domain.Kind { return domain.KindWebhook }

// UsesSigningSecret implements domain.SigningChannel.
func (*Channel) UsesSigningSecret() bool { return true }

func (c *Channel) Prepare(address string) (string, string, error) {
	u, err := safehttp.ParseURL(address, c.Opts)
	if err != nil {
		return "", "", err
	}
	return u.String(), u.Hostname(), nil
}

// Payload is the JSON body.
type Payload struct {
	ID        string            `json:"id"`
	Version   int               `json:"version"`
	Type      string            `json:"type"`
	Severity  notify.Severity   `json:"severity"`
	TenantID  string            `json:"tenant_id"`
	CreatedAt time.Time         `json:"created_at"`
	Title     string            `json:"title"`
	Body      string            `json:"body,omitempty"`
	Link      string            `json:"link,omitempty"`
	Attrs     map[string]string `json:"attrs,omitempty"`
}

// Sign returns the X-Reliabilix-Signature value for a body sent at ts.
func Sign(secret string, ts time.Time, body []byte) string {
	t := strconv.FormatInt(ts.Unix(), 10)
	m := hmac.New(sha256.New, []byte(secret))
	m.Write([]byte(t + "."))
	m.Write(body)
	return "t=" + t + ",v1=" + hex.EncodeToString(m.Sum(nil))
}

func (c *Channel) Send(ctx context.Context, t domain.Target, m domain.Message) error {
	if t.Secret == "" {
		return domain.Permanent(errors.New("no signing secret available"))
	}
	body, err := json.Marshal(Payload{ID: m.DeliveryID, Version: PayloadVersion, Type: m.Event.Type, Severity: m.Event.Severity, TenantID: m.TenantID,
		CreatedAt: m.CreatedAt.UTC(), Title: m.Event.Title, Body: m.Event.Body, Link: m.Event.Link, Attrs: m.Event.Attrs})
	if err != nil {
		return domain.Permanent(err)
	}
	return safehttp.PostJSON(ctx, c.Client, t.Address, map[string]string{
		"User-Agent":             "Reliabilix-Notifications/1",
		"X-Reliabilix-Event":     m.Event.Type,
		"X-Reliabilix-Delivery":  m.DeliveryID,
		"X-Reliabilix-Signature": Sign(t.Secret, time.Now(), body),
	}, body)
}
