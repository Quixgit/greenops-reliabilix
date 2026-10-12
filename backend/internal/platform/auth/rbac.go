// Package auth provides identity claims, RBAC and HTTP middleware.
// Authentication is delegated to Auth0 (ADR-0004); the database is authoritative for who belongs
// to which tenant and with which role, so a role change takes effect immediately.
package auth

import "sort"

// Role is a tenant-scoped role. RoleCI exists only for API keys.
type Role string

const (
	RoleOwner    Role = "owner"
	RoleAdmin    Role = "admin"
	RoleEngineer Role = "engineer"
	RoleViewer   Role = "viewer"
	RoleBilling  Role = "billing"
	RoleCI       Role = "ci"
	RoleAgent    Role = "agent" // API keys of the Kubernetes agent: may push workload energy and nothing else
	RoleSCIM     Role = "scim"  // API keys of an identity provider: may provision members through /scim/v2 and nothing else
	RoleIaC      Role = "iac"   // API keys of infrastructure as code (the Terraform provider): projects, their policies and budgets, nothing else
	// RoleCustom is what a member with an organization-defined role resolves to; the permissions come with the claims.
	RoleCustom Role = "custom"
)

// Permission is a fine-grained capability checked by handlers.
type Permission string

const (
	PermProjectRead      Permission = "project:read"
	PermProjectWrite     Permission = "project:write"
	PermCloudRead        Permission = "cloud:read"
	PermCloudWrite       Permission = "cloud:write"
	PermUsageRead        Permission = "usage:read"
	PermCarbonRead       Permission = "carbon:read"
	PermCarbonCompute    Permission = "carbon:compute"
	PermRecommendRead    Permission = "recommendation:read"
	PermRecommendApply   Permission = "recommendation:apply" // approve, apply and dismiss
	PermReportRead       Permission = "report:read"
	PermReportWrite      Permission = "report:write"
	PermBillingRead      Permission = "billing:read"
	PermBudgetWrite      Permission = "budget:write"
	PermAuditRead        Permission = "audit:read"
	PermTenantAdmin      Permission = "tenant:admin"      // members, invitations, API keys
	PermKubernetesIngest Permission = "kubernetes:ingest" // push workload energy samples (agent keys, owners)
	PermAlertWrite       Permission = "alert:write"       // acknowledge and mute alerts
	PermTenantData       Permission = "tenant:data"       // export all organization data and schedule its erasure: owners only
	PermCIEvaluate       Permission = "ci:evaluate"
	PermSCIM             Permission = "scim:provision" // SCIM user provisioning (identity-provider keys only)

	// Notifications: where events are delivered. Destinations carry addresses and endpoints, so only people who manage
	// the organization change them; engineers can see what is configured and whether deliveries work.
	PermNotificationRead  Permission = "notification:read"
	PermNotificationWrite Permission = "notification:write"

	// Settings: the organization's own configuration. Changing it affects everybody in the organization.
	PermSettingsRead  Permission = "settings:read"
	PermSettingsWrite Permission = "settings:write"

	// Automation: plans for approved recommendations. Machines (CI keys) hold none of these.
	PermAutomationRead    Permission = "automation:read"
	PermAutomationWrite   Permission = "automation:write"   // create a plan, report the result, cancel
	PermAutomationApprove Permission = "automation:approve" // approve a plan for execution
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
	manage = []Permission{PermProjectWrite, PermCloudWrite, PermCarbonCompute, PermReportWrite, PermBillingRead, PermBudgetWrite, PermAuditRead, PermTenantAdmin, PermCIEvaluate, PermNotificationRead, PermNotificationWrite, PermSettingsRead, PermSettingsWrite, PermAlertWrite}
)

var matrix = map[Role]map[Permission]struct{}{
	RoleOwner: perms(read, manage, []Permission{PermTenantData, PermKubernetesIngest, PermRecommendApply, PermAutomationRead, PermAutomationWrite, PermAutomationApprove}),
	// Admin manages people, connections and settings; it can see plans but does not approve infrastructure changes.
	RoleAdmin:    perms(read, manage, []Permission{PermAutomationRead}),
	RoleEngineer: perms(read, []Permission{PermAlertWrite, PermNotificationRead, PermSettingsRead, PermCarbonCompute, PermRecommendApply, PermReportWrite, PermCIEvaluate, PermAutomationRead, PermAutomationWrite, PermAutomationApprove}),
	RoleViewer:   perms(read),
	RoleBilling:  perms([]Permission{PermUsageRead, PermCarbonRead, PermReportRead, PermReportWrite, PermBillingRead, PermBudgetWrite}),
	RoleCI:       perms([]Permission{PermCIEvaluate}),
	RoleAgent:    perms([]Permission{PermKubernetesIngest}),
	RoleSCIM:     perms([]Permission{PermSCIM}),
	// The Terraform provider manages projects, their policies and budgets. It holds no access to cloud connections, members, keys,
	// settings, notifications or automation, and cannot approve anything.
	RoleIaC: perms([]Permission{PermProjectRead, PermProjectWrite, PermBillingRead, PermBudgetWrite}),
}

// Can reports whether the role grants the permission. Unknown roles grant nothing.
func (r Role) Can(p Permission) bool {
	_, ok := matrix[r][p]
	return ok
}

// PermissionInfo describes one permission for the roles editor.
type PermissionInfo struct {
	Key   Permission `json:"key"`
	Group string     `json:"group"`
	// Assignable: may be put in a custom role. Organization administration, data export/erasure and machine-only
	// capabilities stay with the built-in roles so that a custom role can never be a way to gain them.
	Assignable bool `json:"assignable"`
}

// Permissions is the catalog of every permission, grouped for display.
func Permissions() []PermissionInfo {
	g := func(group string, assignable bool, ps ...Permission) []PermissionInfo {
		out := make([]PermissionInfo, 0, len(ps))
		for _, p := range ps {
			out = append(out, PermissionInfo{Key: p, Group: group, Assignable: assignable})
		}
		return out
	}
	var out []PermissionInfo
	for _, part := range [][]PermissionInfo{
		g("projects", true, PermProjectRead, PermProjectWrite),
		g("cloud", true, PermCloudRead, PermCloudWrite),
		g("usage_carbon", true, PermUsageRead, PermCarbonRead, PermCarbonCompute),
		g("recommendations", true, PermRecommendRead, PermRecommendApply),
		g("automation", true, PermAutomationRead, PermAutomationWrite, PermAutomationApprove),
		g("reports", true, PermReportRead, PermReportWrite),
		g("finops", true, PermBillingRead, PermBudgetWrite),
		g("alerts_notifications", true, PermAlertWrite, PermNotificationRead, PermNotificationWrite),
		g("settings_audit", true, PermSettingsRead, PermSettingsWrite, PermAuditRead),
		g("ci", true, PermCIEvaluate),
		g("administration", false, PermTenantAdmin, PermTenantData, PermKubernetesIngest, PermSCIM),
	} {
		out = append(out, part...)
	}
	return out
}

// CustomAssignable reports whether p may be part of a custom role.
func CustomAssignable(p Permission) bool {
	for _, i := range Permissions() {
		if i.Key == p {
			return i.Assignable
		}
	}
	return false
}

// PermissionsOf lists what a built-in role grants, sorted.
func PermissionsOf(r Role) []Permission {
	out := make([]Permission, 0, len(matrix[r]))
	for p := range matrix[r] {
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}
