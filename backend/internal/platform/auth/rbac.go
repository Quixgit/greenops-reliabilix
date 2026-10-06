// Package auth provides identity claims, RBAC and HTTP middleware.
// Authentication is delegated to Auth0 (ADR-0004); the database is authoritative for who belongs
// to which tenant and with which role, so a role change takes effect immediately.
package auth

// Role is a tenant-scoped role. RoleCI exists only for API keys.
type Role string

const (
	RoleOwner    Role = "owner"
	RoleAdmin    Role = "admin"
	RoleEngineer Role = "engineer"
	RoleViewer   Role = "viewer"
	RoleBilling  Role = "billing"
	RoleCI       Role = "ci"
)

// Permission is a fine-grained capability checked by handlers.
type Permission string

const (
	PermProjectRead    Permission = "project:read"
	PermProjectWrite   Permission = "project:write"
	PermCloudRead      Permission = "cloud:read"
	PermCloudWrite     Permission = "cloud:write"
	PermUsageRead      Permission = "usage:read"
	PermCarbonRead     Permission = "carbon:read"
	PermCarbonCompute  Permission = "carbon:compute"
	PermRecommendRead  Permission = "recommendation:read"
	PermRecommendApply Permission = "recommendation:apply" // approve, apply and dismiss
	PermReportRead     Permission = "report:read"
	PermReportWrite    Permission = "report:write"
	PermBillingRead    Permission = "billing:read"
	PermBudgetWrite    Permission = "budget:write"
	PermAuditRead      Permission = "audit:read"
	PermTenantAdmin    Permission = "tenant:admin" // members, invitations, API keys
	PermCIEvaluate     Permission = "ci:evaluate"
)

func perms(groups ...[]Permission) map[Permission]struct{} {
	m := map[Permission]struct{}{}
	for _, g := range groups {
		for _, p := range g {
			m[p] = struct{}{}
		}
	}
	return m
}

var (
	read   = []Permission{PermProjectRead, PermCloudRead, PermUsageRead, PermCarbonRead, PermRecommendRead, PermReportRead}
	manage = []Permission{PermProjectWrite, PermCloudWrite, PermCarbonCompute, PermReportWrite, PermBillingRead, PermBudgetWrite, PermAuditRead, PermTenantAdmin, PermCIEvaluate}
)

var matrix = map[Role]map[Permission]struct{}{
	RoleOwner:    perms(read, manage, []Permission{PermRecommendApply}),
	RoleAdmin:    perms(read, manage), // manages people, connections, settings; does not approve infrastructure changes
	RoleEngineer: perms(read, []Permission{PermCarbonCompute, PermRecommendApply, PermReportWrite, PermCIEvaluate}),
	RoleViewer:   perms(read),
	RoleBilling:  perms([]Permission{PermUsageRead, PermCarbonRead, PermReportRead, PermReportWrite, PermBillingRead, PermBudgetWrite}),
	RoleCI:       perms([]Permission{PermCIEvaluate}),
}

// Can reports whether the role grants the permission. Unknown roles grant nothing.
func (r Role) Can(p Permission) bool {
	_, ok := matrix[r][p]
	return ok
}
