package auth

import "testing"

func TestRoleCan(t *testing.T) {
	cases := []struct {
		role Role
		perm Permission
		want bool
	}{
		{RoleViewer, PermProjectRead, true},
		{RoleViewer, PermProjectWrite, false},
		{RoleEngineer, PermRecommendApply, true},
		{RoleAdmin, PermRecommendApply, false}, // admins manage access, they do not approve infra changes
		{RoleOwner, PermRecommendApply, true},
		{RoleBilling, PermBillingRead, true},
		{RoleBilling, PermProjectRead, false},
		{RoleBilling, PermBudgetWrite, true},
		{RoleOwner, PermTenantAdmin, true},
		{RoleEngineer, PermTenantAdmin, false},
		{RoleCI, PermCIEvaluate, true},
		{RoleCI, PermUsageRead, false},
		{RoleCI, PermProjectRead, false},
		{RoleEngineer, PermAutomationApprove, true},
		{RoleOwner, PermAutomationWrite, true},
		{RoleAdmin, PermAutomationRead, true},
		{RoleAdmin, PermAutomationApprove, false}, // admins see plans, they do not approve infra changes
		{RoleAdmin, PermAutomationWrite, false},
		{RoleViewer, PermAutomationRead, false}, // plans reveal infrastructure details
		{RoleBilling, PermAutomationRead, false},
		{RoleCI, PermAutomationRead, false}, // machines never plan, approve or read automation
		{RoleCI, PermAutomationWrite, false},
		{RoleCI, PermAutomationApprove, false},
		{RoleOwner, PermNotificationWrite, true},
		{RoleAdmin, PermNotificationWrite, true},
		{RoleAdmin, PermNotificationRead, true},
		{RoleEngineer, PermNotificationRead, true},
		{RoleEngineer, PermNotificationWrite, false}, // endpoints and addresses are managed by owners and admins
		{RoleViewer, PermNotificationRead, false},
		{RoleBilling, PermNotificationRead, false},
		{RoleCI, PermNotificationRead, false}, // machines never read or change where events go
		{RoleCI, PermNotificationWrite, false},
		{RoleOwner, PermSettingsWrite, true},
		{RoleAdmin, PermSettingsWrite, true},
		{RoleEngineer, PermSettingsRead, true},
		{RoleEngineer, PermSettingsWrite, false},
		{RoleViewer, PermSettingsRead, false},
		{RoleBilling, PermSettingsRead, false},
		{RoleCI, PermSettingsRead, false},
		{Role("unknown"), PermProjectRead, false},
	}
	for _, c := range cases {
		if got := c.role.Can(c.perm); got != c.want {
			t.Errorf("%s/%s = %v, want %v", c.role, c.perm, got, c.want)
		}
	}
}

func TestSCIMRoleHoldsOnlyProvisioning(t *testing.T) {
	if !RoleSCIM.Can(PermSCIM) {
		t.Fatal("scim keys provision")
	}
	for _, p := range []Permission{PermTenantAdmin, PermProjectRead, PermUsageRead, PermCIEvaluate, PermKubernetesIngest, PermAutomationApprove} {
		if RoleSCIM.Can(p) {
			t.Errorf("scim key must not hold %s", p)
		}
	}
	for _, r := range []Role{RoleOwner, RoleAdmin, RoleEngineer, RoleViewer, RoleBilling, RoleCI, RoleAgent} {
		if r.Can(PermSCIM) {
			t.Errorf("%s must not provision", r)
		}
	}
}
