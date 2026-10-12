package application

import (
	"context"
	"errors"
	"strings"

	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/entitlements"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/settings"
	"github.com/quixgit/greenops-reliabilix/backend/internal/tenants/domain"
)

// ErrSCIMOff: the deployment has no provisioning store.
var ErrSCIMOff = errors.New("provisioning is not available")

// ClaimProvisioned implements auth.Claimer: a first sign-in joins the organization that pushed the verified e-mail.
func (s Service) ClaimProvisioned(ctx context.Context, subject, email string) (string, error) {
	if s.SCIM == nil {
		return "", nil
	}
	return s.SCIM.ClaimProvisioned(ctx, subject, strings.ToLower(strings.TrimSpace(email)))
}

func (s Service) SCIMList(ctx context.Context, tenantID, email string, start, count int) ([]domain.SCIMUser, int, error) {
	if s.SCIM == nil {
		return nil, 0, ErrSCIMOff
	}
	return s.SCIM.ListSCIMUsers(ctx, tenantID, strings.TrimSpace(email), start, count)
}

func (s Service) SCIMGet(ctx context.Context, tenantID, id string) (domain.SCIMUser, error) {
	if s.SCIM == nil {
		return domain.SCIMUser{}, ErrSCIMOff
	}
	return s.SCIM.GetSCIMUser(ctx, tenantID, id)
}

// SCIMCreate follows the same rules as an invitation: a valid e-mail, a role an invitation may grant, the organization's allowed
// domains and its member limit.
func (s Service) SCIMCreate(ctx context.Context, tenantID string, u domain.SCIMUser) (domain.SCIMUser, error) {
	if s.SCIM == nil {
		return domain.SCIMUser{}, ErrSCIMOff
	}
	u.Email = strings.ToLower(strings.TrimSpace(u.Email))
	if err := s.checkSCIM(ctx, tenantID, u); err != nil {
		return domain.SCIMUser{}, err
	}
	if err := s.allow(ctx, tenantID, entitlements.Members); err != nil {
		return domain.SCIMUser{}, err
	}
	return s.SCIM.CreateSCIMUser(ctx, tenantID, u)
}

func (s Service) SCIMUpdate(ctx context.Context, tenantID string, u domain.SCIMUser) (domain.SCIMUser, error) {
	if s.SCIM == nil {
		return domain.SCIMUser{}, ErrSCIMOff
	}
	u.Email = strings.ToLower(strings.TrimSpace(u.Email))
	if err := s.checkSCIM(ctx, tenantID, u); err != nil {
		return domain.SCIMUser{}, err
	}
	return s.SCIM.UpdateSCIMUser(ctx, tenantID, u)
}

func (s Service) SCIMDelete(ctx context.Context, tenantID, id string) error {
	if s.SCIM == nil {
		return ErrSCIMOff
	}
	return s.SCIM.DeleteSCIMUser(ctx, tenantID, id)
}

func (s Service) checkSCIM(ctx context.Context, tenantID string, u domain.SCIMUser) error {
	if !domain.ValidEmail(u.Email) || !domain.ValidSCIMRole(u.Role) || len(u.DisplayName) > 200 || len(u.ExternalID) > 256 {
		return domain.ErrInvalidInput
	}
	if allowed := s.cfg().Strings(ctx, tenantID, settings.KeyInviteAllowedDomain); len(allowed) > 0 && !domain.EmailInDomains(u.Email, allowed) {
		return domain.PolicyError{Reason: "this organization accepts only these e-mail domains: " + strings.Join(allowed, ", ")}
	}
	return nil
}
