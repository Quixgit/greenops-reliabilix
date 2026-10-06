package domain

import (
	"errors"
	"strings"
	"testing"

	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/auth"
)

func TestCheckRoleChange(t *testing.T) {
	cases := []struct {
		name                string
		actor, target, next auth.Role
		owners              int
		want                error
	}{
		{"owner promotes viewer", auth.RoleOwner, auth.RoleViewer, auth.RoleEngineer, 1, nil},
		{"admin promotes viewer", auth.RoleAdmin, auth.RoleViewer, auth.RoleEngineer, 1, nil},
		{"admin cannot grant owner", auth.RoleAdmin, auth.RoleViewer, auth.RoleOwner, 1, ErrForbiddenChange},
		{"admin cannot touch owner", auth.RoleAdmin, auth.RoleOwner, auth.RoleViewer, 2, ErrForbiddenChange},
		{"last owner cannot be demoted", auth.RoleOwner, auth.RoleOwner, auth.RoleAdmin, 1, ErrLastOwner},
		{"owner demotes another owner", auth.RoleOwner, auth.RoleOwner, auth.RoleAdmin, 2, nil},
		{"unknown role", auth.RoleOwner, auth.RoleViewer, "root", 1, ErrInvalidInput},
		{"ci is not a member role", auth.RoleOwner, auth.RoleViewer, auth.RoleCI, 1, ErrInvalidInput},
	}
	for _, c := range cases {
		if err := CheckRoleChange(c.actor, c.target, c.next, c.owners); !errors.Is(err, c.want) && err != c.want {
			t.Errorf("%s: err = %v, want %v", c.name, err, c.want)
		}
	}
}

func TestCheckRemoval(t *testing.T) {
	if err := CheckRemoval(auth.RoleAdmin, auth.RoleViewer, 1); err != nil {
		t.Errorf("admin removes viewer: %v", err)
	}
	if !errors.Is(CheckRemoval(auth.RoleAdmin, auth.RoleOwner, 2), ErrForbiddenChange) {
		t.Error("admin removed an owner")
	}
	if !errors.Is(CheckRemoval(auth.RoleOwner, auth.RoleOwner, 1), ErrLastOwner) {
		t.Error("last owner removed")
	}
	if err := CheckRemoval(auth.RoleOwner, auth.RoleOwner, 2); err != nil {
		t.Errorf("owner removes co-owner: %v", err)
	}
}

func TestRoleSets(t *testing.T) {
	if ValidInviteRole(auth.RoleOwner) || !ValidInviteRole(auth.RoleEngineer) || ValidInviteRole(auth.RoleCI) {
		t.Error("invite roles wrong")
	}
	if !ValidKeyRole(auth.RoleCI) || ValidKeyRole(auth.RoleAdmin) {
		t.Error("key roles wrong")
	}
}

func TestInvitationToken(t *testing.T) {
	tok, hash, err := NewInvitationToken()
	if err != nil || !strings.HasPrefix(tok, "inv_") || hash != auth.HashAPIKey(tok) || strings.Contains(hash, tok) {
		t.Fatalf("token: %v %q %q", err, tok, hash)
	}
	tok2, _, _ := NewInvitationToken()
	if tok == tok2 {
		t.Error("tokens repeat")
	}
}

func TestValidators(t *testing.T) {
	for _, ok := range []string{"a@b.co", "first.last@sub.example.org"} {
		if !ValidEmail(ok) {
			t.Errorf("%q rejected", ok)
		}
	}
	for _, bad := range []string{"", "no-at", "a@b", "a b@c.de", "@x.io"} {
		if ValidEmail(bad) {
			t.Errorf("%q accepted", bad)
		}
	}
	if !ValidName("Acme", 200) || ValidName("   ", 200) || ValidName(strings.Repeat("x", 201), 200) {
		t.Error("ValidName wrong")
	}
}
