// Package email delivers events as plain-text e-mail. Recipients are chosen by the application layer (tenant
// members only), this package makes sure the message itself cannot be abused: addresses and header values are
// stripped of control characters (no header injection) and the body is quoted-printable UTF-8.
package email

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"mime"
	"mime/quotedprintable"
	"net"
	"net/mail"
	"net/smtp"
	"net/textproto"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/quixgit/greenops-reliabilix/backend/internal/notifications/domain"
)

// Sender hands a finished RFC 5322 message to a mail system.
type Sender interface {
	SendMail(ctx context.Context, from, to string, msg []byte) error
}

type Channel struct {
	From   string // envelope and header sender, e.g. "Reliabilix <notifications@example.com>"
	Sender Sender
	Now    func() time.Time
}

func New(from string, s Sender) *Channel { return &Channel{From: from, Sender: s} }

func (*Channel) Kind() domain.Kind { return domain.KindEmail }

// PersonalAddress implements domain.PersonalChannel.
func (*Channel) PersonalAddress() bool { return true }

// Prepare accepts a bare address only ("name@example.com"), lowercases it and masks it for display.
func (*Channel) Prepare(address string) (string, string, error) {
	a := strings.TrimSpace(address)
	if len(a) > 254 || strings.ContainsFunc(a, func(r rune) bool {
		return unicode.IsControl(r) || unicode.IsSpace(r) || r == '<' || r == '>' || r == ',' || r == ';' || r == '"'
	}) {
		return "", "", fmt.Errorf("%w: not a valid e-mail address", domain.ErrInvalid)
	}
	parsed, err := mail.ParseAddress(a)
	if err != nil || parsed.Address != a || strings.Count(a, "@") != 1 {
		return "", "", fmt.Errorf("%w: not a valid e-mail address", domain.ErrInvalid)
	}
	a = strings.ToLower(a)
	return a, Mask(a), nil
}

// Mask shows enough of an address to recognise it without revealing it: "a***@example.com".
func Mask(addr string) string {
	local, dom, ok := strings.Cut(addr, "@")
	if !ok || local == "" {
		return "***"
	}
	return string([]rune(local)[:1]) + "***@" + dom
}

// clean makes a value safe for a header line.
func clean(s string) string {
	return strings.Join(strings.FieldsFunc(s, unicode.IsControl), " ")
}

// Message renders the RFC 5322 message for an event.
func (c *Channel) Message(to string, m domain.Message, now time.Time) ([]byte, string, error) {
	from, err := mail.ParseAddress(c.From)
	if err != nil {
		return nil, "", errors.New("the sender address is not configured correctly")
	}
	var id [12]byte
	_, _ = rand.Read(id[:])
	_, fromDomain, _ := strings.Cut(from.Address, "@")

	var body strings.Builder
	body.WriteString(m.Event.Title + "\n")
	if m.Event.Body != "" {
		body.WriteString("\n" + m.Event.Body + "\n")
	}
	if m.Event.Link != "" {
		body.WriteString("\n" + m.Event.Link + "\n")
	}
	body.WriteString("\n--\nYou receive this because your organization set up e-mail notifications in Reliabilix.\n")

	var buf bytes.Buffer
	h := func(k, v string) { buf.WriteString(k + ": " + v + "\r\n") }
	h("From", from.String())
	h("To", to)
	h("Subject", mime.QEncoding.Encode("utf-8", "[Reliabilix] "+clean(m.Event.Title)))
	h("Date", now.UTC().Format(time.RFC1123Z))
	h("Message-ID", "<"+hex.EncodeToString(id[:])+"@"+fromDomain+">")
	h("MIME-Version", "1.0")
	h("Content-Type", `text/plain; charset="utf-8"`)
	h("Content-Transfer-Encoding", "quoted-printable")
	h("Auto-Submitted", "auto-generated")
	h("X-Reliabilix-Event", clean(m.Event.Type))
	buf.WriteString("\r\n")
	w := quotedprintable.NewWriter(&buf)
	_, _ = w.Write([]byte(strings.ReplaceAll(body.String(), "\n", "\r\n")))
	_ = w.Close()
	return buf.Bytes(), from.Address, nil
}

func (c *Channel) Send(ctx context.Context, t domain.Target, m domain.Message) error {
	if c.Sender == nil {
		return domain.Permanent(errors.New("e-mail is not configured"))
	}
	now := time.Now()
	if c.Now != nil {
		now = c.Now()
	}
	msg, envelopeFrom, err := c.Message(t.Address, m, now)
	if err != nil {
		return domain.Permanent(err)
	}
	return c.Sender.SendMail(ctx, envelopeFrom, t.Address, msg)
}

// SMTP sends through an authenticated, TLS-protected SMTP relay.
type SMTP struct {
	Host, Username, Password string
	Port                     int
	// AllowPlain permits a relay without TLS (a local development mail catcher only).
	AllowPlain bool
}

func (s SMTP) SendMail(ctx context.Context, from, to string, msg []byte) error {
	addr := net.JoinHostPort(s.Host, strconv.Itoa(s.Port))
	d := net.Dialer{Timeout: 10 * time.Second}
	tlsCfg := &tls.Config{ServerName: s.Host, MinVersion: tls.VersionTLS12}

	var conn net.Conn
	var err error
	if s.Port == 465 { // implicit TLS
		conn, err = (&tls.Dialer{NetDialer: &d, Config: tlsCfg}).DialContext(ctx, "tcp", addr)
	} else {
		conn, err = d.DialContext(ctx, "tcp", addr)
	}
	if err != nil {
		return errors.New("could not connect to the mail server")
	}
	defer func() { _ = conn.Close() }()
	_ = conn.SetDeadline(time.Now().Add(30 * time.Second))
	if dl, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(dl)
	}

	c, err := smtp.NewClient(conn, s.Host)
	if err != nil {
		return errors.New("the mail server did not greet us")
	}
	defer func() { _ = c.Close() }()
	if s.Port != 465 {
		if ok, _ := c.Extension("STARTTLS"); ok {
			if err := c.StartTLS(tlsCfg); err != nil {
				return errors.New("TLS handshake with the mail server failed")
			}
		} else if !s.AllowPlain {
			return domain.Permanent(errors.New("the mail server does not offer TLS"))
		}
	}
	if s.Username != "" {
		if err := c.Auth(smtp.PlainAuth("", s.Username, s.Password, s.Host)); err != nil {
			return classify(err, "mail server login failed")
		}
	}
	if err := c.Mail(from); err != nil {
		return classify(err, "sender rejected")
	}
	if err := c.Rcpt(to); err != nil {
		return classify(err, "recipient rejected")
	}
	w, err := c.Data()
	if err != nil {
		return classify(err, "mail server refused the message")
	}
	if _, err := w.Write(msg); err != nil {
		return errors.New("could not send the message")
	}
	if err := w.Close(); err != nil {
		return classify(err, "mail server refused the message")
	}
	return c.Quit()
}

// classify maps an SMTP reply to a delivery outcome: 4xx is temporary, 5xx will not get better by retrying.
func classify(err error, what string) error {
	var te *textproto.Error
	if errors.As(err, &te) {
		if te.Code >= 500 {
			return domain.Permanent(fmt.Errorf("%s (SMTP %d)", what, te.Code))
		}
		return fmt.Errorf("%s (SMTP %d)", what, te.Code)
	}
	return errors.New(what)
}

// Log "sends" by writing a one-line summary to the log. It stands in for SMTP in development.
type Log struct{ L *slog.Logger }

func (l Log) SendMail(_ context.Context, _, to string, msg []byte) error {
	if l.L != nil {
		l.L.Info("e-mail (development: not sent)", "to", Mask(to), "bytes", len(msg))
	}
	return nil
}
