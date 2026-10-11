// Package http exposes tenant-level endpoints: onboarding, members, invitations, API keys, audit log.
package http

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/auth"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/entitlements"
	httpx "github.com/quixgit/greenops-reliabilix/backend/internal/platform/http"
	"github.com/quixgit/greenops-reliabilix/backend/internal/tenants/application"
	"github.com/quixgit/greenops-reliabilix/backend/internal/tenants/domain"
)

type Handlers struct{ Svc application.Service }

// IdentityRoutes are mounted behind identity-only authentication: the caller has a valid token but may not
// belong to a tenant yet.
func (h Handlers) IdentityRoutes(r chi.Router) {
	r.Post("/onboarding", h.onboard)
	r.Post("/invitations/accept", h.acceptInvitation)
}

func (h Handlers) Routes(r chi.Router) {
	admin := auth.Require(auth.PermTenantAdmin)
	r.With(auth.Require(auth.PermProjectRead)).Get("/tenant", h.organization)
	r.With(admin).Patch("/tenant", h.rename)
	r.With(admin).Get("/members", h.members)
	r.With(admin).Patch("/members/{id}", h.changeRole)
	r.With(admin).Delete("/members/{id}", h.removeMember)
	r.With(admin).Get("/roles", h.roles)
	r.With(admin).Post("/roles", h.createRole)
	r.With(admin).Put("/roles/{id}", h.updateRole)
	r.With(admin).Delete("/roles/{id}", h.deleteRole)
	r.With(admin).Get("/invitations", h.invitations)
	r.With(admin).Post("/invitations", h.invite)
	r.With(admin).Get("/api-keys", h.apiKeys)
	r.With(admin).Post("/api-keys", h.createAPIKey)
	r.With(admin).Delete("/api-keys/{id}", h.revokeAPIKey)
	r.With(auth.Require(auth.PermAuditRead)).Get("/audit-log", h.auditLog)
}

func fail(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, domain.ErrNotFound):
		httpx.NotFound(w, r)
	case errors.Is(err, domain.ErrInvalidInput):
		httpx.WriteProblem(w, r, http.StatusUnprocessableEntity, "invalid request", "")
	case errors.Is(err, domain.ErrPermissionNotHeld):
		httpx.WriteProblem(w, r, http.StatusForbidden, "forbidden", err.Error())
	case errors.Is(err, domain.ErrForbiddenChange):
		httpx.WriteProblem(w, r, http.StatusForbidden, "forbidden", err.Error())
	case errors.Is(err, domain.ErrLastOwner):
		httpx.WriteProblem(w, r, http.StatusConflict, "last owner", err.Error())
	case errors.Is(err, domain.ErrInvalidInvitation):
		httpx.WriteProblem(w, r, http.StatusUnprocessableEntity, "invalid or expired invitation", "")
	case errors.Is(err, entitlements.ErrLimitReached):
		httpx.WriteProblem(w, r, http.StatusPaymentRequired, "plan limit reached", err.Error())
	case errors.As(err, new(domain.PolicyError)):
		httpx.WriteProblem(w, r, http.StatusUnprocessableEntity, "not allowed by your organization's settings", err.Error())
	case errors.Is(err, domain.ErrEmailRequired):
		httpx.WriteProblem(w, r, http.StatusForbidden, "verified email required", "email_not_verified")
	default:
		httpx.Internal(w, r, err)
	}
}

func (h Handlers) onboard(w http.ResponseWriter, r *http.Request) {
	c, _ := auth.FromContext(r.Context())
	var in struct {
		OrganizationName string `json:"organization_name"`
	}
	if err := httpx.DecodeJSON(r, &in); err != nil {
		httpx.WriteProblem(w, r, http.StatusBadRequest, "invalid request body", "")
		return
	}
	tenant, err := h.Svc.Onboard(r.Context(), c, in.OrganizationName)
	if err != nil {
		fail(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, map[string]string{"tenant_id": tenant})
}

func (h Handlers) acceptInvitation(w http.ResponseWriter, r *http.Request) {
	c, _ := auth.FromContext(r.Context())
	var in struct {
		Token string `json:"token"`
	}
	if err := httpx.DecodeJSON(r, &in); err != nil {
		httpx.WriteProblem(w, r, http.StatusBadRequest, "invalid request body", "")
		return
	}
	tenant, err := h.Svc.AcceptInvitation(r.Context(), c, in.Token)
	if err != nil {
		fail(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]string{"tenant_id": tenant})
}

func (h Handlers) organization(w http.ResponseWriter, r *http.Request) {
	c, _ := auth.FromContext(r.Context())
	o, err := h.Svc.Organization(r.Context(), c.TenantID)
	if err != nil {
		fail(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, o)
}

// rename takes {"name": "..."}; the database function validates the length again.
func (h Handlers) rename(w http.ResponseWriter, r *http.Request) {
	c, _ := auth.FromContext(r.Context())
	var in struct {
		Name string `json:"name"`
	}
	if err := httpx.DecodeJSON(r, &in); err != nil {
		httpx.WriteProblem(w, r, http.StatusBadRequest, "invalid request body", "")
		return
	}
	o, err := h.Svc.Rename(r.Context(), c.TenantID, in.Name)
	if err != nil {
		fail(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, o)
}

func (h Handlers) members(w http.ResponseWriter, r *http.Request) {
	c, _ := auth.FromContext(r.Context())
	items, err := h.Svc.Members(r.Context(), c.TenantID)
	if err != nil {
		fail(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h Handlers) changeRole(w http.ResponseWriter, r *http.Request) {
	c, _ := auth.FromContext(r.Context())
	id, ok := httpx.PathUUID(r, "id")
	if !ok {
		httpx.NotFound(w, r)
		return
	}
	// Either a built-in role or a custom role id; giving a built-in role also withdraws any custom role.
	var in struct {
		Role         auth.Role `json:"role"`
		CustomRoleID string    `json:"custom_role_id"`
	}
	if err := httpx.DecodeJSON(r, &in); err != nil || (in.Role == "") == (in.CustomRoleID == "") {
		httpx.WriteProblem(w, r, http.StatusBadRequest, "invalid request body", "give either role or custom_role_id")
		return
	}
	var m domain.Member
	var err error
	if in.CustomRoleID != "" {
		if !httpx.ParseUUID(in.CustomRoleID) {
			httpx.WriteProblem(w, r, http.StatusBadRequest, "invalid request body", "")
			return
		}
		m, err = h.Svc.AssignCustomRole(r.Context(), c.TenantID, id, in.CustomRoleID, c)
	} else {
		m, err = h.Svc.ChangeRole(r.Context(), c.TenantID, id, in.Role, c)
	}
	if err != nil {
		fail(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, m)
}

// roles answers the catalog of permissions, what each built-in role grants and the organization's custom roles.
func (h Handlers) roles(w http.ResponseWriter, r *http.Request) {
	c, _ := auth.FromContext(r.Context())
	custom, err := h.Svc.CustomRoles(r.Context(), c.TenantID)
	if err != nil {
		fail(w, r, err)
		return
	}
	type builtin struct {
		Role        auth.Role         `json:"role"`
		Permissions []auth.Permission `json:"permissions"`
	}
	var bs []builtin
	for _, role := range []auth.Role{auth.RoleOwner, auth.RoleAdmin, auth.RoleEngineer, auth.RoleBilling, auth.RoleViewer} {
		bs = append(bs, builtin{Role: role, Permissions: auth.PermissionsOf(role)})
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"permissions": auth.Permissions(), "builtin": bs, "custom": custom})
}

type roleBody struct {
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Permissions []auth.Permission `json:"permissions"`
}

func (h Handlers) createRole(w http.ResponseWriter, r *http.Request) {
	c, _ := auth.FromContext(r.Context())
	var in roleBody
	if err := httpx.DecodeJSON(r, &in); err != nil {
		httpx.WriteProblem(w, r, http.StatusBadRequest, "invalid request body", "")
		return
	}
	x, err := h.Svc.CreateCustomRole(r.Context(), c.TenantID, c, in.Name, in.Description, in.Permissions)
	if err != nil {
		fail(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, x)
}

func (h Handlers) updateRole(w http.ResponseWriter, r *http.Request) {
	c, _ := auth.FromContext(r.Context())
	id, ok := httpx.PathUUID(r, "id")
	if !ok {
		httpx.NotFound(w, r)
		return
	}
	var in roleBody
	if err := httpx.DecodeJSON(r, &in); err != nil {
		httpx.WriteProblem(w, r, http.StatusBadRequest, "invalid request body", "")
		return
	}
	x, err := h.Svc.UpdateCustomRole(r.Context(), c.TenantID, id, c, in.Name, in.Description, in.Permissions)
	if err != nil {
		fail(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, x)
}

func (h Handlers) deleteRole(w http.ResponseWriter, r *http.Request) {
	c, _ := auth.FromContext(r.Context())
	id, ok := httpx.PathUUID(r, "id")
	if !ok {
		httpx.NotFound(w, r)
		return
	}
	if err := h.Svc.DeleteCustomRole(r.Context(), c.TenantID, id, c); err != nil {
		fail(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h Handlers) removeMember(w http.ResponseWriter, r *http.Request) {
	c, _ := auth.FromContext(r.Context())
	id, ok := httpx.PathUUID(r, "id")
	if !ok {
		httpx.NotFound(w, r)
		return
	}
	if err := h.Svc.RemoveMember(r.Context(), c.TenantID, id, c); err != nil {
		fail(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h Handlers) invitations(w http.ResponseWriter, r *http.Request) {
	c, _ := auth.FromContext(r.Context())
	items, err := h.Svc.Invitations(r.Context(), c.TenantID)
	if err != nil {
		fail(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h Handlers) invite(w http.ResponseWriter, r *http.Request) {
	c, _ := auth.FromContext(r.Context())
	var in struct {
		Email string    `json:"email"`
		Role  auth.Role `json:"role"`
	}
	if err := httpx.DecodeJSON(r, &in); err != nil {
		httpx.WriteProblem(w, r, http.StatusBadRequest, "invalid request body", "")
		return
	}
	inv, token, err := h.Svc.Invite(r.Context(), c.TenantID, c, in.Email, in.Role)
	if err != nil {
		fail(w, r, err)
		return
	}
	// The token is shown once; share it with the invitee (no email delivery in phase 1).
	httpx.WriteJSON(w, http.StatusCreated, map[string]any{"invitation": inv, "token": token})
}

func (h Handlers) apiKeys(w http.ResponseWriter, r *http.Request) {
	c, _ := auth.FromContext(r.Context())
	items, err := h.Svc.APIKeys(r.Context(), c.TenantID)
	if err != nil {
		fail(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h Handlers) createAPIKey(w http.ResponseWriter, r *http.Request) {
	c, _ := auth.FromContext(r.Context())
	var in struct {
		Name string    `json:"name"`
		Role auth.Role `json:"role"`
	}
	if err := httpx.DecodeJSON(r, &in); err != nil {
		httpx.WriteProblem(w, r, http.StatusBadRequest, "invalid request body", "")
		return
	}
	k, key, err := h.Svc.CreateAPIKey(r.Context(), c.TenantID, c, in.Name, in.Role)
	if err != nil {
		fail(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, map[string]any{"api_key": k, "key": key}) // the key is shown once
}

func (h Handlers) revokeAPIKey(w http.ResponseWriter, r *http.Request) {
	c, _ := auth.FromContext(r.Context())
	id, ok := httpx.PathUUID(r, "id")
	if !ok {
		httpx.NotFound(w, r)
		return
	}
	if err := h.Svc.RevokeAPIKey(r.Context(), c.TenantID, id); err != nil {
		fail(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h Handlers) auditLog(w http.ResponseWriter, r *http.Request) {
	c, _ := auth.FromContext(r.Context())
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit < 1 || limit > 200 {
		limit = 50
	}
	items, err := h.Svc.AuditLog(r.Context(), c.TenantID, r.URL.Query().Get("action"), limit)
	if err != nil {
		fail(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"items": items})
}
