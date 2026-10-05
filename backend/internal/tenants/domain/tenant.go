// Package domain defines organizations (tenants), memberships, invitations and API keys, with the rules
// that keep access control safe (last-owner protection, who may grant which role).
package domain

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"regexp"
	"strings"
	"time"

	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/auth"
)

type Organization struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Plan      string    `json:"plan"`
	CreatedAt time.Time `json:"created_at"`
}

type Member struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	Subject   string    `json:"-"`
	Email     string    `json:"email"`
	Role      auth.Role `json:"role"`
	CreatedAt time.Time `json:"created_at"`
}

type Invitation struct {
	ID         string     `json:"id"`
	Email      string     `json:"email"`
	Role       auth.Role  `json:"role"`
	CreatedBy  string     `json:"created_by"`
	ExpiresAt  time.Time  `json:"expires_at"`
	AcceptedAt *time.Time `json:"accepted_at"`
	CreatedAt  time.Time  `json:"created_at"`
}

type APIKey struct {
	ID         string     `json:"id"`
	Name       string     `json:"name"`
	Prefix     string     `json:"prefix"`
	Role       auth.Role  `json:"role"`
	CreatedBy  string     `json:"created_by"`
	CreatedAt  time.Time  `json:"created_at"`
	LastUsedAt *time.Time `json:"last_used_at"`
	RevokedAt  *time.Time `json:"revoked_at"`
}

type AuditEntry struct {
	ID        int64           `json:"id"`
	Actor     string          `json:"actor_user_id"`
	Action    string          `json:"action"`
	Target    string          `json:"target"`
	RequestID string          `json:"request_id"`
	Metadata  json.RawMessage `json:"metadata"`
	CreatedAt time.Time       `json:"created_at"`
}

var (
	ErrInvalidInput      = errors.New("invalid input")
	ErrNotFound          = errors.New("not found")
	ErrForbiddenChange   = errors.New("you may not change this member")
	ErrLastOwner         = errors.New("the organization must keep at least one owner")
	ErrInvalidInvitation = errors.New("invalid or expired invitation")
	ErrEmailRequired     = errors.New("a verified email is required")
)

var assignable = map[auth.Role]bool{auth.RoleOwner: true, auth.RoleAdmin: true, auth.RoleEngineer: true, auth.RoleViewer: true, auth.RoleBilling: true}

// ValidRole reports whether r is a role a member can hold.
func ValidRole(r auth.Role) bool { return assignable[r] }

// ValidInviteRole: owners are created by role change from an existing owner, never by invitation.
func ValidInviteRole(r auth.Role) bool { return assignable[r] && r != auth.RoleOwner }

// ValidKeyRole: API keys are machine credentials with minimal roles.
func ValidKeyRole(r auth.Role) bool { return r == auth.RoleCI || r == auth.RoleViewer }

var emailRe = regexp.MustCompile(`^[^@\s]{1,64}@[^@\s]{1,190}\.[^@\s]{2,}$`)

func ValidEmail(s string) bool { return len(s) <= 254 && emailRe.MatchString(s) }

func ValidName(s string, max int) bool {
	n := len([]rune(strings.TrimSpace(s)))
	return n >= 1 && n <= max
}

// CheckRoleChange decides whether actor may set a member (currently target) to newRole.
// Only owners may touch owners or grant ownership; the last owner can never be demoted.
func CheckRoleChange(actor, target, newRole auth.Role, owners int) error {
	if !ValidRole(newRole) {
		return ErrInvalidInput
	}
	if (target == auth.RoleOwner || newRole == auth.RoleOwner) && actor != auth.RoleOwner {
		return ErrForbiddenChange
	}
	if target == auth.RoleOwner && newRole != auth.RoleOwner && owners <= 1 {
		return ErrLastOwner
	}
	return nil
}

// CheckRemoval decides whether actor may remove a member.
func CheckRemoval(actor, target auth.Role, owners int) error {
	if target == auth.RoleOwner {
		if actor != auth.RoleOwner {
			return ErrForbiddenChange
		}
		if owners <= 1 {
			return ErrLastOwner
		}
	}
	return nil
}

// NewInvitationToken returns a one-time token and its storage hash (only the hash is stored).
func NewInvitationToken() (token, hash string, err error) {
	b := make([]byte, 32)
	if _, err = rand.Read(b); err != nil {
		return "", "", err
	}
	token = "inv_" + base64.RawURLEncoding.EncodeToString(b)
	return token, auth.HashAPIKey(token), nil
}

// Repository is tenant-scoped (explicit tenantID + RLS) except the identity-resolution functions.
type Repository interface {
	// identity (SECURITY DEFINER functions)
	Resolve(ctx context.Context, subject, wantedTenant string) (tenantID string, role auth.Role, err error)
	LookupAPIKey(ctx context.Context, hash string) (tenantID string, role auth.Role, keyID string, err error)
	Onboard(ctx context.Context, subject, email, orgName string) (tenantID string, err error)
	AcceptInvitation(ctx context.Context, tokenHash, subject, email string) (tenantID string, err error)
	// tenant data
	Organization(ctx context.Context, tenantID string) (Organization, error)
	Members(ctx context.Context, tenantID string) ([]Member, error)
	ChangeRole(ctx context.Context, tenantID, memberID string, newRole auth.Role, actor auth.Claims) (Member, error)
	RemoveMember(ctx context.Context, tenantID, memberID string, actor auth.Claims) error
	CreateInvitation(ctx context.Context, tenantID string, inv Invitation, tokenHash string) (Invitation, error)
	Invitations(ctx context.Context, tenantID string) ([]Invitation, error)
	CreateAPIKey(ctx context.Context, tenantID string, k APIKey, hash string) (APIKey, error)
	APIKeys(ctx context.Context, tenantID string) ([]APIKey, error)
	RevokeAPIKey(ctx context.Context, tenantID, id string) error
	AuditLog(ctx context.Context, tenantID, action string, limit int) ([]AuditEntry, error)
}
