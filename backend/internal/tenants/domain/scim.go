package domain

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/auth"
)

// SCIMUser is a person an identity provider pushed (ADR-0056). Joined is true once they have signed in and are a member.
type SCIMUser struct {
	ID          string    `json:"id"`
	ExternalID  string    `json:"external_id"`
	Email       string    `json:"email"`
	DisplayName string    `json:"display_name"`
	Role        auth.Role `json:"role"`
	Active      bool      `json:"active"`
	Joined      bool      `json:"joined"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// ErrConflict: the e-mail is already provisioned.
var ErrConflict = errors.New("already exists")

// SCIMStore is tenant-scoped like the rest of the repository.
type SCIMStore interface {
	// ListSCIMUsers returns one page (start is 1-based) and the total; email, when set, is an exact case-insensitive match.
	ListSCIMUsers(ctx context.Context, tenantID, email string, start, count int) ([]SCIMUser, int, error)
	GetSCIMUser(ctx context.Context, tenantID, id string) (SCIMUser, error)
	CreateSCIMUser(ctx context.Context, tenantID string, u SCIMUser) (SCIMUser, error)
	// UpdateSCIMUser replaces the person's attributes and applies the effect on the membership (role change, removal on deactivation).
	UpdateSCIMUser(ctx context.Context, tenantID string, u SCIMUser) (SCIMUser, error)
	// DeleteSCIMUser deprovisions: the membership goes and so does the record.
	DeleteSCIMUser(ctx context.Context, tenantID, id string) error
	// ClaimProvisioned attaches a first sign-in to the organization that pushed this verified e-mail ("" when none).
	ClaimProvisioned(ctx context.Context, subject, email string) (string, error)
}

// ValidSCIMRole: a person can be provisioned as anything an invitation can grant (never owner).
func ValidSCIMRole(r auth.Role) bool { return ValidInviteRole(r) }

// SCIMRoleFrom picks the role from the SCIM `roles` values: the first one that is valid; viewer when none.
func SCIMRoleFrom(values []string) auth.Role {
	for _, v := range values {
		if r := auth.Role(strings.ToLower(strings.TrimSpace(v))); ValidSCIMRole(r) {
			return r
		}
	}
	return auth.RoleViewer
}
