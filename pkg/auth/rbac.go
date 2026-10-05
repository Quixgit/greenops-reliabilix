// Package auth provides identity claims, RBAC and HTTP middleware.
// Authentication itself is delegated to an external OIDC provider (ADR-0008);
// services only verify tokens and enforce authorization.
package auth

// Role is a tenant-scoped role.
type Role string

const (
	RoleOwner    Role = "owner"
	RoleAdmin    Role = "admin"
	RoleEngineer Role = "engineer"
	RoleViewer   Role = "viewer"
	RoleBilling  Role = "billing"
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
	PermRecommendApply Permission = "recommendation:apply"
	PermReportRead     Permission = "report:read"
	PermBillingRead    Permission = "billing:read"
	PermAuditRead      Permission = "audit:read"
	PermTenantAdmin    Permission = "tenant:admin"
)

var read = []Permission{PermProjectRead, PermCloudRead, PermUsageRead, PermCarbonRead, PermRecommendRead, PermReportRead}

var matrix = map[Role]map[Permission]struct{}{
	RoleOwner:    set(append(read, PermProjectWrite, PermCloudWrite, PermCarbonCompute, PermRecommendApply, PermBillingRead, PermAuditRead, PermTenantAdmin)...),
	RoleAdmin:    set(append(read, PermProjectWrite, PermCloudWrite, PermCarbonCompute, PermAuditRead, PermTenantAdmin)...),
	RoleEngineer: set(append(read, PermCarbonCompute, PermRecommendApply)...),
	RoleViewer:   set(read...),
	RoleBilling:  set(PermUsageRead, PermCarbonRead, PermReportRead, PermBillingRead),
}

func set(p ...Permission) map[Permission]struct{} {
	m := make(map[Permission]struct{}, len(p))
	for _, x := range p {
		m[x] = struct{}{}
	}
	return m
}

// Can reports whether the role grants the permission. Unknown roles grant nothing.
func (r Role) Can(p Permission) bool {
	_, ok := matrix[r][p]
	return ok
}
