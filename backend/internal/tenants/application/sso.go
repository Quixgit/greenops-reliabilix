package application

import (
	"context"
	"net"
	"strings"

	"github.com/quixgit/greenops-reliabilix/backend/internal/tenants/domain"
)

func (s Service) resolver() domain.TXTResolver {
	if s.DNS != nil {
		return s.DNS
	}
	return net.DefaultResolver
}

func (s Service) SSODomains(ctx context.Context, tenantID string) ([]domain.SSODomain, error) {
	if s.SSO == nil {
		return []domain.SSODomain{}, nil
	}
	return s.SSO.ListSSODomains(ctx, tenantID)
}

// AddSSODomain claims a domain for the organization. It is not in force until the DNS record is published and verified.
func (s Service) AddSSODomain(ctx context.Context, tenantID, actor, dom, connection, jitRole string) (domain.SSODomain, error) {
	if s.SSO == nil {
		return domain.SSODomain{}, ErrSCIMOff
	}
	d, err := domain.NormalizeDomain(dom)
	if err != nil {
		return domain.SSODomain{}, err
	}
	connection = strings.TrimSpace(connection)
	if !domain.ValidConnection(connection) {
		return domain.SSODomain{}, domain.ErrConnectionName
	}
	if !domain.ValidJITRole(jitRole) {
		return domain.SSODomain{}, domain.ErrInvalidJITRole
	}
	token, err := domain.NewDomainToken()
	if err != nil {
		return domain.SSODomain{}, err
	}
	return s.SSO.CreateSSODomain(ctx, tenantID, d, connection, jitRole, token, actor)
}

// VerifySSODomain looks for the proof in DNS and, if it is there, puts the domain in force.
func (s Service) VerifySSODomain(ctx context.Context, tenantID, id string) error {
	if s.SSO == nil {
		return ErrSCIMOff
	}
	dom, token, verified, err := s.SSO.SSODomainToken(ctx, tenantID, id)
	if err != nil {
		return err
	}
	if verified {
		return nil
	}
	if !domain.CheckProof(ctx, s.resolver(), dom, token) {
		return domain.ErrNotVerified
	}
	return s.SSO.MarkSSODomainVerified(ctx, tenantID, id)
}

func (s Service) DeleteSSODomain(ctx context.Context, tenantID, id string) error {
	if s.SSO == nil {
		return ErrSCIMOff
	}
	return s.SSO.DeleteSSODomain(ctx, tenantID, id)
}

// SSOConnection tells the sign-in page where an address signs in (empty: the ordinary sign-in).
func (s Service) SSOConnection(ctx context.Context, email string) (string, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if s.SSO == nil || len(email) > 254 || !domain.ValidEmail(email) {
		return "", nil
	}
	return s.SSO.ConnectionFor(ctx, email)
}
