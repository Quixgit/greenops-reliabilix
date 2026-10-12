package application

import (
	"context"
	"errors"
	"testing"

	"github.com/quixgit/greenops-reliabilix/backend/internal/tenants/domain"
)

type memSSO struct {
	rows     map[string]*domain.SSODomain
	tokens   map[string]string
	verified map[string]bool
	owner    map[string]string // verified domain -> tenant
	claimed  []string
	seq      int
}

func newMemSSO() *memSSO {
	return &memSSO{rows: map[string]*domain.SSODomain{}, tokens: map[string]string{}, verified: map[string]bool{}, owner: map[string]string{}}
}
func (m *memSSO) ListSSODomains(context.Context, string) ([]domain.SSODomain, error) { return nil, nil }
func (m *memSSO) CreateSSODomain(_ context.Context, tenant, dom, conn, jit, token, _ string) (domain.SSODomain, error) {
	m.seq++
	id := "id" + string(rune('0'+m.seq))
	d := &domain.SSODomain{ID: id, Domain: dom, Connection: conn, JITRole: jit}
	d.RecordName, d.RecordValue = domain.ProofRecord(dom, token)
	m.rows[id], m.tokens[id] = d, token
	m.owner[id] = tenant
	return *d, nil
}
func (m *memSSO) SSODomainToken(_ context.Context, _, id string) (string, string, bool, error) {
	d, ok := m.rows[id]
	if !ok {
		return "", "", false, domain.ErrNotFound
	}
	return d.Domain, m.tokens[id], m.verified[id], nil
}
func (m *memSSO) MarkSSODomainVerified(_ context.Context, tenant, id string) error {
	for other, d := range m.rows {
		if other != id && d.Domain == m.rows[id].Domain && m.verified[other] {
			return domain.ErrDomainTaken
		}
	}
	m.verified[id] = true
	return nil
}
func (m *memSSO) DeleteSSODomain(_ context.Context, _, id string) error {
	if _, ok := m.rows[id]; !ok {
		return domain.ErrNotFound
	}
	delete(m.rows, id)
	return nil
}
func (m *memSSO) ConnectionFor(_ context.Context, email string) (string, error) {
	for id, d := range m.rows {
		if m.verified[id] && len(email) > len(d.Domain) && email[len(email)-len(d.Domain)-1:] == "@"+d.Domain {
			return d.Connection, nil
		}
	}
	return "", nil
}
func (m *memSSO) ClaimDomain(_ context.Context, sub, email string) (string, error) {
	m.claimed = append(m.claimed, sub+"/"+email)
	return "", nil
}

type dnsFake map[string][]string

func (d dnsFake) LookupTXT(_ context.Context, n string) ([]string, error) {
	if v, ok := d[n]; ok {
		return v, nil
	}
	return nil, errors.New("nxdomain")
}

func TestADomainIsInForceOnlyAfterItsRecordIsPublished(t *testing.T) {
	st, dns := newMemSSO(), dnsFake{}
	s := Service{SSO: st, DNS: dns}
	ctx := context.Background()
	d, err := s.AddSSODomain(ctx, "T1", "admin", " Acme.COM ", "acme-saml", "viewer")
	if err != nil || d.Domain != "acme.com" || d.RecordName != "_reliabilix-sso.acme.com" {
		t.Fatalf("%+v %v", d, err)
	}
	if c, _ := s.SSOConnection(ctx, "ann@acme.com"); c != "" {
		t.Fatalf("an unverified domain routes nobody: %q", c)
	}
	if err := s.VerifySSODomain(ctx, "T1", d.ID); !errors.Is(err, domain.ErrNotVerified) {
		t.Fatalf("no record yet: %v", err)
	}
	dns[d.RecordName] = []string{d.RecordValue}
	if err := s.VerifySSODomain(ctx, "T1", d.ID); err != nil {
		t.Fatal(err)
	}
	if c, _ := s.SSOConnection(ctx, "Ann@Acme.com"); c != "acme-saml" {
		t.Fatalf("a verified domain routes its addresses to the connection: %q", c)
	}
	if c, _ := s.SSOConnection(ctx, "ann@other.com"); c != "" {
		t.Fatalf("another domain: %q", c)
	}
	if c, _ := s.SSOConnection(ctx, "not-an-email"); c != "" {
		t.Fatal("a malformed address is not asked")
	}
}

func TestOnlyTheOrganizationThatProvesADomainKeepsIt(t *testing.T) {
	st, dns := newMemSSO(), dnsFake{}
	s := Service{SSO: st, DNS: dns}
	ctx := context.Background()
	a, _ := s.AddSSODomain(ctx, "T1", "x", "acme.com", "c1", "")
	b, _ := s.AddSSODomain(ctx, "T2", "y", "acme.com", "c2", "")
	dns[a.RecordName] = []string{a.RecordValue, b.RecordValue}
	if err := s.VerifySSODomain(ctx, "T1", a.ID); err != nil {
		t.Fatal(err)
	}
	if err := s.VerifySSODomain(ctx, "T2", b.ID); !errors.Is(err, domain.ErrDomainTaken) {
		t.Fatalf("the second claimant must be refused: %v", err)
	}
}

func TestBadDomainsConnectionsAndRolesAreRefused(t *testing.T) {
	s := Service{SSO: newMemSSO(), DNS: dnsFake{}}
	ctx := context.Background()
	if _, err := s.AddSSODomain(ctx, "T", "a", "gmail.com", "c", ""); !errors.Is(err, domain.ErrDomainInvalid) {
		t.Errorf("public mail: %v", err)
	}
	if _, err := s.AddSSODomain(ctx, "T", "a", "acme.com", "bad name!", ""); !errors.Is(err, domain.ErrConnectionName) {
		t.Errorf("connection: %v", err)
	}
	if _, err := s.AddSSODomain(ctx, "T", "a", "acme.com", "c", "owner"); !errors.Is(err, domain.ErrInvalidJITRole) {
		t.Errorf("owner by sign-in must never be possible: %v", err)
	}
	if _, err := (Service{}).AddSSODomain(ctx, "T", "a", "acme.com", "c", ""); err == nil {
		t.Error("without a store nothing is offered")
	}
}

type scimStub struct{ joined string }

func (scimStub) ListSCIMUsers(context.Context, string, string, int, int) ([]domain.SCIMUser, int, error) {
	return nil, 0, nil
}
func (scimStub) GetSCIMUser(context.Context, string, string) (domain.SCIMUser, error) { return domain.SCIMUser{}, nil }
func (scimStub) CreateSCIMUser(context.Context, string, domain.SCIMUser) (domain.SCIMUser, error) {
	return domain.SCIMUser{}, nil
}
func (scimStub) UpdateSCIMUser(context.Context, string, domain.SCIMUser) (domain.SCIMUser, error) {
	return domain.SCIMUser{}, nil
}
func (scimStub) DeleteSCIMUser(context.Context, string, string) error { return nil }
func (s scimStub) ClaimProvisioned(context.Context, string, string) (string, error) { return s.joined, nil }

func TestFirstSignInTriesProvisioningBeforeTheDomain(t *testing.T) {
	st := newMemSSO()
	s := Service{SCIM: scimStub{joined: "T-scim"}, SSO: st}
	if got, _ := s.ClaimProvisioned(context.Background(), "sub", "Ann@acme.com"); got != "T-scim" || len(st.claimed) != 0 {
		t.Fatalf("a person the directory pushed joins that organization: %q, domain asked %v", got, st.claimed)
	}
	s.SCIM = scimStub{}
	if got, _ := s.ClaimProvisioned(context.Background(), "sub", "Ann@Acme.com"); got != "" || len(st.claimed) != 1 || st.claimed[0] != "sub/ann@acme.com" {
		t.Fatalf("otherwise the domain is asked, with the address in lower case: %q %v", got, st.claimed)
	}
}
