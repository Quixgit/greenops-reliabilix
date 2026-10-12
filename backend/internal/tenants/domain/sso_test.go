package domain

import (
	"context"
	"errors"
	"testing"
)

func TestNormalizeDomain(t *testing.T) {
	for in, want := range map[string]string{"Acme.COM": "acme.com", " acme.co.uk. ": "acme.co.uk", "sub.acme-corp.io": "sub.acme-corp.io"} {
		if got, err := NormalizeDomain(in); err != nil || got != want {
			t.Errorf("NormalizeDomain(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	for _, in := range []string{"", "acme", "gmail.com", "Outlook.com", "a@b.com", "1.2.3.4", "-acme.com", "acme..com", "ac me.com", "https://acme.com"} {
		if _, err := NormalizeDomain(in); !errors.Is(err, ErrDomainInvalid) {
			t.Errorf("NormalizeDomain(%q) must be refused, got %v", in, err)
		}
	}
}

type fakeDNS map[string][]string

func (f fakeDNS) LookupTXT(_ context.Context, name string) ([]string, error) {
	v, ok := f[name]
	if !ok {
		return nil, errors.New("no such host")
	}
	return v, nil
}

func TestCheckProofNeedsTheExactRecordUnderTheRightName(t *testing.T) {
	name, value := ProofRecord("acme.com", "abc123")
	if name != "_reliabilix-sso.acme.com" || value != "reliabilix-sso=abc123" {
		t.Fatalf("%q %q", name, value)
	}
	ctx := context.Background()
	if !CheckProof(ctx, fakeDNS{name: {"v=spf1 -all", `"reliabilix-sso=abc123"`}}, "acme.com", "abc123") {
		t.Error("the record among others, in quotes, must be found")
	}
	for label, dns := range map[string]fakeDNS{
		"wrong token": {name: {"reliabilix-sso=other"}}, "prefix only": {name: {"reliabilix-sso=abc1234"}}, "other name": {"acme.com": {value}}, "no record": {},
	} {
		if CheckProof(ctx, dns, "acme.com", "abc123") {
			t.Errorf("%s must not verify", label)
		}
	}
}

func TestSSOValidators(t *testing.T) {
	if !ValidConnection("acme-saml") || !ValidConnection("Acme_SSO.1") || ValidConnection("a b") || ValidConnection("") || ValidConnection("-x") {
		t.Error("connection names")
	}
	if !ValidJITRole("") || !ValidJITRole("viewer") || ValidJITRole("owner") || ValidJITRole("admin") {
		t.Error("jit roles: never owner or admin")
	}
	a, _ := NewDomainToken()
	b, _ := NewDomainToken()
	if len(a) != 32 || a == b {
		t.Errorf("tokens %q %q", a, b)
	}
}
