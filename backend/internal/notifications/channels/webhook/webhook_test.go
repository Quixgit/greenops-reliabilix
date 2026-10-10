package webhook

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/quixgit/greenops-reliabilix/backend/internal/notifications/channels/safehttp"
	"github.com/quixgit/greenops-reliabilix/backend/internal/notifications/domain"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/notify"
)

func TestPrepare(t *testing.T) {
	c := New(safehttp.Options{})
	addr, hint, err := c.Prepare("https://hooks.example.com/in/abc123?token=s3cret")
	if err != nil || addr != "https://hooks.example.com/in/abc123?token=s3cret" || hint != "hooks.example.com" {
		t.Errorf("Prepare = %q, %q, %v", addr, hint, err)
	}
	if strings.Contains(hint, "s3cret") || strings.Contains(hint, "abc123") {
		t.Error("the hint must only show the host")
	}
	if _, _, err := c.Prepare("http://hooks.example.com/x"); err == nil {
		t.Error("http must be rejected")
	}
	if _, _, err := c.Prepare("https://10.0.0.1/x"); err == nil {
		t.Error("private addresses must be rejected")
	}
}

func TestSendSignsAndDescribesTheEvent(t *testing.T) {
	var gotBody []byte
	var gotHdr http.Header
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotBody, _ = io.ReadAll(r.Body)
		gotHdr = r.Header.Clone()
	}))
	defer srv.Close()
	c := New(safehttp.Options{AllowPrivate: true, AllowHTTP: true})
	created := time.Date(2026, 10, 9, 12, 0, 0, 0, time.UTC)
	err := c.Send(context.Background(), domain.Target{DestinationID: "d1", Address: srv.URL, Secret: "topsecret"}, domain.Message{
		DeliveryID: "del-1", TenantID: "t-1", CreatedAt: created,
		Event: notify.Event{Type: notify.TypeReportReady, Severity: notify.Info, Title: "Report ready", Body: "csv", Link: "https://app/reports", Attrs: map[string]string{"report_id": "r1"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	var p Payload
	if err := json.Unmarshal(gotBody, &p); err != nil {
		t.Fatal(err)
	}
	if p.ID != "del-1" || p.Version != 1 || p.Type != "report.ready" || p.TenantID != "t-1" || p.Title != "Report ready" || p.Attrs["report_id"] != "r1" || !p.CreatedAt.Equal(created) {
		t.Errorf("payload = %+v", p)
	}
	if gotHdr.Get("X-Reliabilix-Event") != "report.ready" || gotHdr.Get("X-Reliabilix-Delivery") != "del-1" || !strings.HasPrefix(gotHdr.Get("User-Agent"), "Reliabilix") {
		t.Errorf("headers = %v", gotHdr)
	}
	// A receiver verifies the signature exactly like this.
	parts := strings.Split(gotHdr.Get("X-Reliabilix-Signature"), ",")
	if len(parts) != 2 || !strings.HasPrefix(parts[0], "t=") || !strings.HasPrefix(parts[1], "v1=") {
		t.Fatalf("signature header = %q", gotHdr.Get("X-Reliabilix-Signature"))
	}
	m := hmac.New(sha256.New, []byte("topsecret"))
	m.Write([]byte(strings.TrimPrefix(parts[0], "t=") + "." + string(gotBody)))
	if strings.TrimPrefix(parts[1], "v1=") != hex.EncodeToString(m.Sum(nil)) {
		t.Error("signature does not verify with the shared secret")
	}
}

func TestSendRefusesWithoutSecret(t *testing.T) {
	err := New(safehttp.Options{AllowPrivate: true, AllowHTTP: true}).Send(context.Background(), domain.Target{Address: "http://127.0.0.1:1"}, domain.Message{})
	if err == nil || !domain.IsPermanent(err) {
		t.Errorf("an unsigned webhook must never be sent, got %v", err)
	}
}

func TestSignDependsOnSecretTimeAndBody(t *testing.T) {
	ts := time.Unix(1_700_000_000, 0)
	a := Sign("s", ts, []byte("body"))
	if a != Sign("s", ts, []byte("body")) {
		t.Error("Sign must be deterministic")
	}
	for name, b := range map[string]string{"secret": Sign("x", ts, []byte("body")), "time": Sign("s", ts.Add(time.Second), []byte("body")), "body": Sign("s", ts, []byte("other"))} {
		if a == b {
			t.Errorf("signature must change with the %s", name)
		}
	}
}
