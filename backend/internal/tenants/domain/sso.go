package domain

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net"
	"regexp"
	"strings"
	"time"
)

// SSODomain is an e-mail domain an organization claims for company sign-in (ADR-0064).
type SSODomain struct {
	ID         string     `json:"id"`
	Domain     string     `json:"domain"`
	Connection string     `json:"connection"`
	JITRole    string     `json:"jit_role"` // "" = people must be invited or provisioned
	Verified   bool       `json:"verified"`
	VerifiedAt *time.Time `json:"verified_at"`
	CreatedAt  time.Time  `json:"created_at"`
	// Record is what the DNS TXT record must say while the domain is not verified.
	RecordName  string `json:"record_name,omitempty"`
	RecordValue string `json:"record_value,omitempty"`
}

var (
	ErrDomainTaken    = errors.New("this domain is already verified by another organization")
	ErrDomainInvalid  = errors.New("not a company domain")
	ErrNotVerified    = errors.New("the DNS record was not found")
	ErrConnectionName = errors.New("invalid connection name")
	ErrInvalidJITRole = errors.New("jit_role must be viewer, engineer, billing or empty")
	domainRe          = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]*[a-z0-9])?(\.[a-z0-9]([a-z0-9-]*[a-z0-9])?)+$`)
	connectionRe      = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]{0,99}$`)
	publicMailDomains = map[string]bool{"gmail.com": true, "googlemail.com": true, "outlook.com": true, "hotmail.com": true, "live.com": true, "yahoo.com": true, "icloud.com": true,
		"me.com": true, "aol.com": true, "proton.me": true, "protonmail.com": true, "gmx.com": true, "yandex.com": true, "yandex.ru": true, "mail.ru": true, "qq.com": true}
)

// RecordPrefix is the DNS label under which the proof is published.
const RecordPrefix = "_reliabilix-sso."

// NormalizeDomain lower-cases, drops a trailing dot, and refuses what is not a domain of a company: an address, an IP, a single label, a public mail provider.
func NormalizeDomain(s string) (string, error) {
	d := strings.TrimSuffix(strings.ToLower(strings.TrimSpace(s)), ".")
	if len(d) > 253 || !domainRe.MatchString(d) || net.ParseIP(d) != nil || publicMailDomains[d] {
		return "", ErrDomainInvalid
	}
	return d, nil
}

func ValidConnection(c string) bool { return connectionRe.MatchString(c) }

func ValidJITRole(r string) bool {
	return r == "" || r == "viewer" || r == "engineer" || r == "billing"
}

// NewDomainToken is the secret value of the proof (not a credential: it only proves control of the DNS zone).
func NewDomainToken() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// ProofRecord is the DNS name and value for a domain and token.
func ProofRecord(domain, token string) (name, value string) {
	return RecordPrefix + domain, "reliabilix-sso=" + token
}

// TXTResolver is net.Resolver's lookup (replaced in tests).
type TXTResolver interface {
	LookupTXT(ctx context.Context, name string) ([]string, error)
}

// CheckProof says whether the domain publishes the proof. A lookup failure is "not found", never an error to the person beyond that.
func CheckProof(ctx context.Context, r TXTResolver, domain, token string) bool {
	ctx, cancel := context.WithTimeout(ctx, 6*time.Second)
	defer cancel()
	name, want := ProofRecord(domain, token)
	recs, err := r.LookupTXT(ctx, name)
	if err != nil {
		return false
	}
	for _, rec := range recs {
		if strings.TrimSpace(strings.Trim(rec, `"`)) == want {
			return true
		}
	}
	return false
}

// SSOStore is tenant-scoped like the rest of the repository.
type SSOStore interface {
	ListSSODomains(ctx context.Context, tenantID string) ([]SSODomain, error)
	CreateSSODomain(ctx context.Context, tenantID, domain, connection, jitRole, token, actor string) (SSODomain, error)
	// SSODomainToken returns the unverified domain and the token its record must carry.
	SSODomainToken(ctx context.Context, tenantID, id string) (domain, token string, verified bool, err error)
	MarkSSODomainVerified(ctx context.Context, tenantID, id string) error
	DeleteSSODomain(ctx context.Context, tenantID, id string) error
	// ConnectionFor is the sign-in connection of a verified domain ("" when there is none). It needs no organization: the sign-in page asks it.
	ConnectionFor(ctx context.Context, email string) (string, error)
	// ClaimDomain joins a first sign-in to the organization whose verified domain allows it ("" when none).
	ClaimDomain(ctx context.Context, subject, email string) (string, error)
}
