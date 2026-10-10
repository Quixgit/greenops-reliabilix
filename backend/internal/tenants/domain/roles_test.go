package domain

import (
	"errors"
	"testing"

	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/auth"
)

func admin() auth.Claims { return auth.Claims{Subject: "a", TenantID: "t", Role: auth.RoleAdmin} }

func TestCustomRoleCannotCarryAdministrationOrPermissionsTheActorLacks(t *testing.T) {
	read := []auth.Permission{auth.PermProjectRead, auth.PermCarbonRead}
	if err := ValidateCustomRole("Analyst", "", read, admin()); err != nil {
		t.Fatalf("a plain role must be accepted: %v", err)
	}
	for name, perms := range map[string][]auth.Permission{
		"tenant admin": {auth.PermProjectRead, auth.PermTenantAdmin},
		"data export":  {auth.PermTenantData},
		"machine only": {auth.PermKubernetesIngest},
		"unknown":      {"nope:nope"},
		"duplicate":    {auth.PermProjectRead, auth.PermProjectRead},
		"empty":        {},
	} {
		if err := ValidateCustomRole("Analyst", "", perms, admin()); !errors.Is(err, ErrInvalidInput) {
			t.Errorf("%s = %v, want ErrInvalidInput", name, err)
		}
	}
	// an admin does not hold automation:approve, so cannot hand it out through a role
	if err := ValidateCustomRole("Approver", "", []auth.Permission{auth.PermAutomationApprove}, admin()); !errors.Is(err, ErrPermissionNotHeld) {
		t.Errorf("approve by admin = %v, want ErrPermissionNotHeld", err)
	}
	owner := auth.Claims{Subject: "o", TenantID: "t", Role: auth.RoleOwner}
	if err := ValidateCustomRole("Approver", "", []auth.Permission{auth.PermAutomationApprove}, owner); err != nil {
		t.Errorf("an owner may: %v", err)
	}
	for _, bad := range []string{"", "x", string(make([]byte, 61))} {
		if err := ValidateCustomRole(bad, "", read, owner); !errors.Is(err, ErrInvalidInput) {
			t.Errorf("name %q = %v", bad, err)
		}
	}
}

func TestClaimsUseTheCustomSetNotTheBuiltInRole(t *testing.T) {
	c := auth.Claims{Role: auth.RoleCustom, Permissions: map[auth.Permission]struct{}{auth.PermCarbonRead: {}}}
	if !c.Can(auth.PermCarbonRead) || c.Can(auth.PermProjectWrite) || c.Can(auth.PermTenantAdmin) {
		t.Error("a custom role grants exactly its set")
	}
	empty := auth.Claims{Role: auth.RoleCustom, Permissions: map[auth.Permission]struct{}{}}
	if empty.Can(auth.PermProjectRead) {
		t.Error("a custom role whose set could not be loaded must grant nothing")
	}
	if !(auth.Claims{Role: auth.RoleViewer}).Can(auth.PermProjectRead) {
		t.Error("a built-in role still works")
	}
}
