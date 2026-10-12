// Package application holds the tenant use-cases: onboarding, members, invitations, API keys, audit.
package application

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/auth"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/entitlements"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/settings"
	"github.com/quixgit/greenops-reliabilix/backend/internal/tenants/domain"
)

type Service struct {
	Repo domain.Repository
	Now  func() time.Time
	// Settings and Limits are optional: without them the catalog defaults apply and nothing is limited.
	Settings settings.Reader
	Limits   entitlements.Gate
	// SCIM is the store of people an identity provider pushed; without it provisioning is not offered.
	SCIM domain.SCIMStore
	// Mail and BaseURL deliver invitations by e-mail (optional: without them the token is shown to the inviter only).
	Mail    InviteMailer
	BaseURL string
	// SSO keeps the company sign-in domains (ADR-0064); DNS looks up their proof (net.DefaultResolver when nil).
	SSO domain.SSOStore
	DNS domain.TXTResolver
}

func (s Service) cfg() settings.Reader {
	if s.Settings != nil {
		return s.Settings
	}
	return settings.Defaults{}
}

func (s Service) allow(ctx context.Context, tenantID string, r entitlements.Resource) error {
	if s.Limits == nil {
		return nil
	}
	return s.Limits.Allow(ctx, tenantID, r)
}

func (s Service) now() time.Time {
	if s.Now != nil {
		return s.Now().UTC()
	}
	return time.Now().UTC()
}

// Resolve implements auth.Resolver.
func (s Service) Resolve(ctx context.Context, subject, wantedTenant string) (string, auth.Role, error) {
	return s.Repo.Resolve(ctx, subject, wantedTenant)
}

// LookupAPIKey implements auth.APIKeyLookup.
func (s Service) LookupAPIKey(ctx context.Context, hash string) (string, auth.Role, string, error) {
	return s.Repo.LookupAPIKey(ctx, hash)
}

// Onboard creates the caller's organization (idempotent: an existing member gets their tenant back).
// The email comes from the verified token, never from the request body.
func (s Service) Onboard(ctx context.Context, who auth.Claims, orgName string) (string, error) {
	orgName = strings.TrimSpace(orgName)
	if !domain.ValidName(orgName, 200) || !domain.ValidEmail(who.Email) {
		return "", domain.ErrInvalidInput
	}
	return s.Repo.Onboard(ctx, who.Subject, strings.ToLower(who.Email), orgName)
}

// AcceptInvitation joins the caller to the inviting tenant. The invited address must equal the caller's
// verified email, so a leaked token alone is useless.
func (s Service) AcceptInvitation(ctx context.Context, who auth.Claims, token string) (string, error) {
	if !who.EmailVerified || !domain.ValidEmail(who.Email) {
		return "", domain.ErrEmailRequired
	}
	if !strings.HasPrefix(token, "inv_") || len(token) > 100 {
		return "", domain.ErrInvalidInvitation
	}
	return s.Repo.AcceptInvitation(ctx, auth.HashAPIKey(token), who.Subject, strings.ToLower(who.Email))
}

// Rename changes the organization's name. Only administrators reach it (the route requires tenant:admin).
func (s Service) Rename(ctx context.Context, tenantID, name string) (domain.Organization, error) {
	return s.Repo.Rename(ctx, tenantID, name)
}

func (s Service) Organization(ctx context.Context, tenantID string) (domain.Organization, error) {
	return s.Repo.Organization(ctx, tenantID)
}

func (s Service) Members(ctx context.Context, tenantID string) ([]domain.Member, error) {
	return s.Repo.Members(ctx, tenantID)
}

func (s Service) ChangeRole(ctx context.Context, tenantID, memberID string, role auth.Role, actor auth.Claims) (domain.Member, error) {
	return s.Repo.ChangeRole(ctx, tenantID, memberID, role, actor)
}

func (s Service) RemoveMember(ctx context.Context, tenantID, memberID string, actor auth.Claims) error {
	return s.Repo.RemoveMember(ctx, tenantID, memberID, actor)
}

// Invite creates a one-time invitation. The token is returned once; only its hash is stored. Delivery is
// the caller's job (no email service in phase 1).
func (s Service) Invite(ctx context.Context, tenantID string, actor auth.Claims, email string, role auth.Role) (domain.Invitation, string, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if !domain.ValidEmail(email) || !domain.ValidInviteRole(role) {
		return domain.Invitation{}, "", domain.ErrInvalidInput
	}
	if allowed := s.cfg().Strings(ctx, tenantID, settings.KeyInviteAllowedDomain); len(allowed) > 0 && !domain.EmailInDomains(email, allowed) {
		return domain.Invitation{}, "", domain.PolicyError{Reason: "invitations are limited to these e-mail domains: " + strings.Join(allowed, ", ")}
	}
	if err := s.allow(ctx, tenantID, entitlements.Members); err != nil {
		return domain.Invitation{}, "", err
	}
	token, hash, err := domain.NewInvitationToken()
	if err != nil {
		return domain.Invitation{}, "", err
	}
	ttl := time.Duration(s.cfg().Int(ctx, tenantID, settings.KeyInviteExpiryDays)) * 24 * time.Hour
	inv, err := s.Repo.CreateInvitation(ctx, tenantID, domain.Invitation{Email: email, Role: role, CreatedBy: actor.Subject, ExpiresAt: s.now().Add(ttl)}, hash)
	return inv, token, err
}

func (s Service) Invitations(ctx context.Context, tenantID string) ([]domain.Invitation, error) {
	return s.Repo.Invitations(ctx, tenantID)
}

// CreateAPIKey issues a machine credential. The key is returned once; only its hash is stored.
func (s Service) CreateAPIKey(ctx context.Context, tenantID string, actor auth.Claims, name string, role auth.Role) (domain.APIKey, string, error) {
	name = strings.TrimSpace(name)
	if !domain.ValidName(name, 100) || !domain.ValidKeyRole(role) {
		return domain.APIKey{}, "", domain.ErrInvalidInput
	}
	existing, err := s.Repo.APIKeys(ctx, tenantID)
	if err != nil {
		return domain.APIKey{}, "", err
	}
	active := int64(0)
	for _, k := range existing {
		if k.RevokedAt == nil {
			active++
		}
	}
	if max := s.cfg().Int(ctx, tenantID, settings.KeyAPIKeysMaxActive); active >= max {
		return domain.APIKey{}, "", domain.PolicyError{Reason: "your organization allows at most " + strconv.FormatInt(max, 10) + " active API keys; revoke one first"}
	}
	key, prefix, err := auth.GenerateAPIKey()
	if err != nil {
		return domain.APIKey{}, "", err
	}
	k, err := s.Repo.CreateAPIKey(ctx, tenantID, domain.APIKey{Name: name, Prefix: prefix, Role: role, CreatedBy: actor.Subject}, auth.HashAPIKey(key))
	return k, key, err
}

func (s Service) APIKeys(ctx context.Context, tenantID string) ([]domain.APIKey, error) {
	return s.Repo.APIKeys(ctx, tenantID)
}
func (s Service) RevokeAPIKey(ctx context.Context, tenantID, id string) error {
	return s.Repo.RevokeAPIKey(ctx, tenantID, id)
}

func (s Service) AuditLog(ctx context.Context, tenantID, action string, limit int) ([]domain.AuditEntry, error) {
	return s.Repo.AuditLog(ctx, tenantID, action, limit)
}

// MemberPermissions implements auth.PermissionResolver.
func (s Service) MemberPermissions(ctx context.Context, subject, tenantID string) ([]auth.Permission, error) {
	return s.Repo.MemberPermissions(ctx, subject, tenantID)
}

func (s Service) CustomRoles(ctx context.Context, tenantID string) ([]domain.CustomRole, error) {
	return s.Repo.CustomRoles(ctx, tenantID)
}

func (s Service) CreateCustomRole(ctx context.Context, tenantID string, actor auth.Claims, name, description string, perms []auth.Permission) (domain.CustomRole, error) {
	if err := domain.ValidateCustomRole(name, description, perms, actor); err != nil {
		return domain.CustomRole{}, err
	}
	return s.Repo.CreateCustomRole(ctx, tenantID, domain.CustomRole{Name: strings.TrimSpace(name), Description: strings.TrimSpace(description), Permissions: perms}, actor)
}

func (s Service) UpdateCustomRole(ctx context.Context, tenantID, id string, actor auth.Claims, name, description string, perms []auth.Permission) (domain.CustomRole, error) {
	if err := domain.ValidateCustomRole(name, description, perms, actor); err != nil {
		return domain.CustomRole{}, err
	}
	// Editing a role is granting what it contains to everyone who holds it: the actor must hold all of it, old and new.
	cur, err := s.Repo.CustomRole(ctx, tenantID, id)
	if err != nil {
		return domain.CustomRole{}, err
	}
	for _, p := range cur.Permissions {
		if !actor.Can(p) {
			return domain.CustomRole{}, domain.ErrPermissionNotHeld
		}
	}
	return s.Repo.UpdateCustomRole(ctx, tenantID, id, domain.CustomRole{Name: strings.TrimSpace(name), Description: strings.TrimSpace(description), Permissions: perms}, actor)
}

func (s Service) DeleteCustomRole(ctx context.Context, tenantID, id string, actor auth.Claims) error {
	return s.Repo.DeleteCustomRole(ctx, tenantID, id, actor)
}

// AssignCustomRole gives a member a custom role; the actor must hold every permission of that role.
func (s Service) AssignCustomRole(ctx context.Context, tenantID, memberID, roleID string, actor auth.Claims) (domain.Member, error) {
	role, err := s.Repo.CustomRole(ctx, tenantID, roleID)
	if err != nil {
		return domain.Member{}, err
	}
	for _, p := range role.Permissions {
		if !actor.Can(p) {
			return domain.Member{}, domain.ErrPermissionNotHeld
		}
	}
	return s.Repo.AssignCustomRole(ctx, tenantID, memberID, roleID, actor)
}
