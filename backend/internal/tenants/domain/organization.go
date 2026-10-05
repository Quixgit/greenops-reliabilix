// Package domain defines organizations (tenants) and memberships.
package domain

import "github.com/quixgit/greenops-reliabilix/backend/internal/platform/auth"

type Organization struct {
	ID   string
	Name string
	Plan string
	// WhiteLabel is phase 3; stored as opaque JSON.
	WhiteLabel []byte
}

type Membership struct {
	OrganizationID string
	UserID         string
	Role           auth.Role
}

// ValidRole reports whether r is an assignable role.
func ValidRole(r auth.Role) bool {
	switch r {
	case auth.RoleOwner, auth.RoleAdmin, auth.RoleEngineer, auth.RoleViewer, auth.RoleBilling:
		return true
	}
	return false
}
