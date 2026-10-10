package email

import (
	"context"
	"errors"
	"io"
	"mime"
	"mime/quotedprintable"
	"net/mail"
	"net/textproto"
	"strings"
	"testing"
	"time"

	"github.com/quixgit/greenops-reliabilix/backend/internal/notifications/domain"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/notify"
)

func TestPrepare(t *testing.T) {
	c := New("Reliabilix <n@example.com>", nil)
	addr, hint, err := c.Prepare("  Alice.Smith@Example.COM ")
	if err != nil || addr != "alice.smith@example.com" || hint != "a***@example.com" {
		t.Errorf("Prepare = %q, %q, %v", addr, hint, err)
	}
	for _, bad := range []string{
		"", "plain", "a@b@c.com", "Alice <alice@example.com>", "a@example.com, b@example.com", "a@example.com;b@example.com",
		"a@example.com\r\nBcc: victim@example.com", "a b@example.com", "<a@example.com>", `"a"@example.com`, strings.Repeat("a", 250) + "@example.com",
	} {
		if _, _, err := c.Prepare(bad); !errors.Is(err, domain.ErrInvalid) {
			t.Errorf("Prepare(%q) = %v, want ErrInvalid", bad, err)
		}
	}
}

func TestMessageIsSafeAndWellFormed(t *testing.T) {
	c := New("Reliabilix <notifications@example.com>", nil)
	now := time.Date(2026, 10, 9, 12, 0, 0, 0, time.UTC)
	raw, from, err := c.Message("alice@example.com", domain.Message{Event: notify.Event{
		Type: notify.TypeReportReady, Severity: notify.Info,
		Title: "Report ready\r\nBcc: victim@example.com", Body: "Дані готові", Link: "https://app.example.com/reports",
	}}, now)
	if err != nil || from != "notifications@example.com" {
		t.Fatalf("Message: from=%q err=%v", from, err)
	}
	m, err := mail.ReadMessage(strings.NewReader(string(raw)))
	if err != nil {
		t.Fatal(err)
	}
	if m.Header.Get("Bcc") != "" || len(m.Header["Bcc"]) != 0 {
		t.Fatal("header injection through the title")
	}
	subj, err := new(mime.WordDecoder).DecodeHeader(m.Header.Get("Subject"))
	if err != nil || !strings.HasPrefix(subj, "[Reliabilix] Report ready") || strings.ContainsAny(subj, "\r\n") {
		t.Errorf("subject = %q (%v)", subj, err)
	}
	if m.Header.Get("To") != "alice@example.com" || m.Header.Get("Auto-Submitted") != "auto-generated" || m.Header.Get("Message-Id") == "" || !strings.HasSuffix(m.Header.Get("Message-Id"), "@example.com>") {
		t.Errorf("headers = %v", m.Header)
	}
	body, _ := io.ReadAll(quotedprintable.NewReader(m.Body))
	for _, want := range []string{"Дані готові", "https://app.example.com/reports", "Report ready"} {
		if !strings.Contains(string(body), want) {
			t.Errorf("body lacks %q:\n%s", want, body)
		}
	}
}

func TestMessageRejectsBadSender(t *testing.T) {
	if _, _, err := New("not an address", nil).Message("a@example.com", domain.Message{}, time.Now()); err == nil {
		t.Error("a malformed sender must fail")
	}
}

type fakeSender struct {
	to, from string
	msg      []byte
	err      error
}

func (f *fakeSender) SendMail(_ context.Context, from, to string, msg []byte) error {
	f.from, f.to, f.msg = from, to, msg
	return f.err
}

func TestSendUsesTheSender(t *testing.T) {
	fs := &fakeSender{}
	c := New("Reliabilix <n@example.com>", fs)
	err := c.Send(context.Background(), domain.Target{Address: "bob@example.com"}, domain.Message{Event: notify.Event{Type: notify.TypeReportReady, Severity: notify.Info, Title: "Hi"}})
	if err != nil || fs.to != "bob@example.com" || fs.from != "n@example.com" || len(fs.msg) == 0 {
		t.Errorf("Send: err=%v sender=%+v", err, fs)
	}
	if err := New("n@example.com", nil).Send(context.Background(), domain.Target{Address: "bob@example.com"}, domain.Message{}); !domain.IsPermanent(err) {
		t.Errorf("no sender configured must be permanent, got %v", err)
	}
}

func TestClassifySMTPReplies(t *testing.T) {
	if err := classify(&textproto.Error{Code: 550, Msg: "no such user"}, "recipient rejected"); !domain.IsPermanent(err) {
		t.Errorf("550 must be permanent: %v", err)
	}
	if err := classify(&textproto.Error{Code: 451, Msg: "try later"}, "x"); err == nil || domain.IsPermanent(err) {
		t.Errorf("451 must be transient: %v", err)
	}
	if err := classify(errors.New("EOF"), "x"); err == nil || domain.IsPermanent(err) {
		t.Errorf("network errors must be transient: %v", err)
	}
}

func TestMask(t *testing.T) {
	for in, want := range map[string]string{"alice@example.com": "a***@example.com", "x@y.io": "x***@y.io", "bad": "***", "@x.io": "***"} {
		if got := Mask(in); got != want {
			t.Errorf("Mask(%q) = %q, want %q", in, got, want)
		}
	}
}
