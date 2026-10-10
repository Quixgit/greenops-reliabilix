// Package safehttp is the HTTP client for outbound calls to tenant-supplied URLs (webhooks, Slack). A tenant
// chooses where the platform connects, so every connection is treated as hostile input (SSRF):
//   - https only (plain http only when explicitly allowed, for development),
//   - the address is checked when the connection is made, after DNS resolution, so a hostname that resolves or
//     rebinds to a private, loopback, link-local or metadata address is refused,
//   - redirects are never followed, proxies from the environment are ignored,
//   - request time and response size are bounded, and errors never include the URL (it may carry a token).
package safehttp

import (
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"syscall"
	"time"

	"github.com/quixgit/greenops-reliabilix/backend/internal/notifications/domain"
)

// Options relax the rules for local development and tests only.
type Options struct {
	AllowPrivate bool // allow loopback / private addresses (a local receiver)
	AllowHTTP    bool // allow http:// URLs
}

var errBlocked = errors.New("destination address is not allowed")

// ParseURL validates a tenant-supplied endpoint.
func ParseURL(raw string, o Options) (*url.URL, error) {
	if len(raw) > 2048 || strings.ContainsAny(raw, "\r\n\t ") {
		return nil, fmt.Errorf("%w: not a valid URL", domain.ErrInvalid)
	}
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" || u.Hostname() == "" {
		return nil, fmt.Errorf("%w: not a valid URL", domain.ErrInvalid)
	}
	switch {
	case u.Scheme == "http" && o.AllowHTTP:
	case u.Scheme != "https":
		return nil, fmt.Errorf("%w: the URL must use https", domain.ErrInvalid)
	}
	if u.User != nil {
		return nil, fmt.Errorf("%w: credentials in the URL are not allowed", domain.ErrInvalid)
	}
	if u.Fragment != "" {
		return nil, fmt.Errorf("%w: the URL must not contain a fragment", domain.ErrInvalid)
	}
	if !o.AllowPrivate {
		h := strings.ToLower(u.Hostname())
		if h == "localhost" || strings.HasSuffix(h, ".localhost") || strings.HasSuffix(h, ".internal") || strings.HasSuffix(h, ".local") {
			return nil, fmt.Errorf("%w: %v", domain.ErrInvalid, errBlocked)
		}
		if ip, err := netip.ParseAddr(h); err == nil && !isPublic(ip) {
			return nil, fmt.Errorf("%w: %v", domain.ErrInvalid, errBlocked)
		}
	}
	return u, nil
}

var nonPublic = mustPrefixes(
	"0.0.0.0/8", "100.64.0.0/10", "192.0.0.0/24", "192.0.2.0/24", "198.18.0.0/15", "198.51.100.0/24", "203.0.113.0/24",
	"240.0.0.0/4", "64:ff9b::/96", "100::/64", "2001:db8::/32", "fc00::/7",
)

func mustPrefixes(ss ...string) []netip.Prefix {
	out := make([]netip.Prefix, len(ss))
	for i, s := range ss {
		out[i] = netip.MustParsePrefix(s)
	}
	return out
}

// isPublic reports whether ip is a globally routable unicast address.
func isPublic(ip netip.Addr) bool {
	ip = ip.Unmap()
	if !ip.IsValid() || !ip.IsGlobalUnicast() || ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() || ip.IsMulticast() || ip.IsUnspecified() {
		return false
	}
	for _, p := range nonPublic {
		if p.Contains(ip) {
			return false
		}
	}
	return true
}

// NewClient builds the client. Its dialer refuses non-public addresses unless o.AllowPrivate.
func NewClient(o Options) *http.Client {
	d := &net.Dialer{Timeout: 5 * time.Second}
	if !o.AllowPrivate {
		d.Control = func(_, address string, _ syscall.RawConn) error {
			host, _, err := net.SplitHostPort(address)
			if err != nil {
				return errBlocked
			}
			ip, err := netip.ParseAddr(host)
			if err != nil || !isPublic(ip) {
				return errBlocked
			}
			return nil
		}
	}
	return &http.Client{
		Timeout: 15 * time.Second,
		Transport: &http.Transport{
			Proxy:                 nil,
			DialContext:           d.DialContext,
			TLSClientConfig:       &tls.Config{MinVersion: tls.VersionTLS12},
			TLSHandshakeTimeout:   5 * time.Second,
			ResponseHeaderTimeout: 10 * time.Second,
			MaxIdleConns:          20,
			IdleConnTimeout:       60 * time.Second,
			DisableKeepAlives:     false,
		},
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
}

// PostJSON posts body and classifies the outcome: nil on 2xx, domain.Permanent for responses that retrying will
// not fix, a plain error for transient problems. Messages never contain the URL.
func PostJSON(ctx context.Context, c *http.Client, target string, headers map[string]string, body []byte) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, target, bytes.NewReader(body))
	if err != nil {
		return domain.Permanent(errors.New("invalid endpoint"))
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := c.Do(req)
	if err != nil {
		return describe(err)
	}
	defer func() { _ = resp.Body.Close() }()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 64<<10))
	return Classify(resp.StatusCode)
}

// Classify maps an HTTP status to a delivery outcome.
func Classify(status int) error {
	switch {
	case status >= 200 && status < 300:
		return nil
	case status == http.StatusRequestTimeout || status == http.StatusTooManyRequests || status >= 500:
		return fmt.Errorf("endpoint answered HTTP %d", status)
	case status >= 300 && status < 400:
		return domain.Permanent(fmt.Errorf("endpoint answered HTTP %d: redirects are not followed", status))
	default:
		return domain.Permanent(fmt.Errorf("endpoint rejected the request with HTTP %d", status))
	}
}

// describe turns a transport error into a safe, short reason (net/http errors embed the URL).
func describe(err error) error {
	var ne net.Error
	var dns *net.DNSError
	switch {
	case errors.Is(err, errBlocked):
		return domain.Permanent(errBlocked)
	case errors.As(err, &dns):
		return errors.New("host name could not be resolved")
	case errors.Is(err, context.DeadlineExceeded), errors.As(err, &ne) && ne.Timeout():
		return errors.New("request timed out")
	case strings.Contains(err.Error(), "certificate") || strings.Contains(err.Error(), "tls:"):
		return errors.New("TLS handshake failed")
	case errors.Is(err, syscall.ECONNREFUSED):
		return errors.New("connection refused")
	default:
		return errors.New("request failed")
	}
}
