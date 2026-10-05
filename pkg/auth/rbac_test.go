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
		{RoleAdmin, PermRecommendApply, false},
		{RoleBilling, PermBillingRead, true},
		{RoleBilling, PermProjectRead, false},
		{RoleOwner, PermTenantAdmin, true},
		{Role("unknown"), PermProjectRead, false},
	}
	for _, c := range cases {
		if got := c.role.Can(c.perm); got != c.want {
			t.Errorf("%s/%s = %v, want %v", c.role, c.perm, got, c.want)
		}
	}
}
