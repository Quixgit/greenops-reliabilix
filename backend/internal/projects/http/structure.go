package http

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/auth"
	httpx "github.com/quixgit/greenops-reliabilix/backend/internal/platform/http"
	"github.com/quixgit/greenops-reliabilix/backend/internal/projects/domain"
)

// StructureRoutes registers teams, environments, allocation rules and the allocation report.
func (h Handlers) StructureRoutes(r chi.Router) {
	read, write := auth.Require(auth.PermProjectRead), auth.Require(auth.PermProjectWrite)
	r.With(read).Get("/teams", h.listTeams)
	r.With(write).Post("/teams", h.createTeam)
	r.With(write).Delete("/teams/{id}", h.deleteTeam)
	r.With(write).Put("/projects/{id}/team", h.setProjectTeam)

	r.With(read).Get("/environments", h.listEnvironments)
	r.With(write).Post("/environments", h.createEnvironment)
	r.With(write).Delete("/environments/{id}", h.deleteEnvironment)
	r.With(write).Put("/environments/{id}/connections", h.setEnvironmentConnections)

	r.With(read).Get("/allocation-rules", h.listRules)
	r.With(write).Post("/allocation-rules", h.createRule)
	r.With(write).Delete("/allocation-rules/{id}", h.deleteRule)
	r.With(read).Get("/allocation", h.allocation)
}

func (h Handlers) tenant(r *http.Request) string {
	c, _ := auth.FromContext(r.Context())
	return c.TenantID
}

func (h Handlers) listTeams(w http.ResponseWriter, r *http.Request) {
	items, err := h.Svc.ListTeams(r.Context(), h.tenant(r))
	if err != nil {
		fail(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h Handlers) createTeam(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Name string `json:"name"`
	}
	if err := httpx.DecodeJSON(r, &in); err != nil {
		httpx.WriteProblem(w, r, http.StatusBadRequest, "invalid request body", "")
		return
	}
	t, err := h.Svc.CreateTeam(r.Context(), h.tenant(r), in.Name)
	if err != nil {
		fail(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, t)
}

func (h Handlers) deleteTeam(w http.ResponseWriter, r *http.Request) {
	id, ok := httpx.PathUUID(r, "id")
	if !ok {
		httpx.NotFound(w, r)
		return
	}
	if err := h.Svc.DeleteTeam(r.Context(), h.tenant(r), id); err != nil {
		fail(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h Handlers) setProjectTeam(w http.ResponseWriter, r *http.Request) {
	id, ok := httpx.PathUUID(r, "id")
	if !ok {
		httpx.NotFound(w, r)
		return
	}
	var in struct {
		TeamID *string `json:"team_id"`
	}
	if err := httpx.DecodeJSON(r, &in); err != nil {
		httpx.WriteProblem(w, r, http.StatusBadRequest, "invalid request body", "")
		return
	}
	if err := h.Svc.SetProjectTeam(r.Context(), h.tenant(r), id, in.TeamID); err != nil {
		fail(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"project_id": id, "team_id": in.TeamID})
}

func (h Handlers) listEnvironments(w http.ResponseWriter, r *http.Request) {
	items, err := h.Svc.ListEnvironments(r.Context(), h.tenant(r))
	if err != nil {
		fail(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h Handlers) createEnvironment(w http.ResponseWriter, r *http.Request) {
	var in struct {
		ProjectID string `json:"project_id"`
		Name      string `json:"name"`
		Kind      string `json:"kind"`
	}
	if err := httpx.DecodeJSON(r, &in); err != nil {
		httpx.WriteProblem(w, r, http.StatusBadRequest, "invalid request body", "")
		return
	}
	e, err := h.Svc.CreateEnvironment(r.Context(), h.tenant(r), in.ProjectID, in.Name, in.Kind)
	if err != nil {
		fail(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, e)
}

func (h Handlers) deleteEnvironment(w http.ResponseWriter, r *http.Request) {
	id, ok := httpx.PathUUID(r, "id")
	if !ok {
		httpx.NotFound(w, r)
		return
	}
	if err := h.Svc.DeleteEnvironment(r.Context(), h.tenant(r), id); err != nil {
		fail(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h Handlers) setEnvironmentConnections(w http.ResponseWriter, r *http.Request) {
	id, ok := httpx.PathUUID(r, "id")
	if !ok {
		httpx.NotFound(w, r)
		return
	}
	var in struct {
		ConnectionIDs []string `json:"connection_ids"`
	}
	if err := httpx.DecodeJSON(r, &in); err != nil {
		httpx.WriteProblem(w, r, http.StatusBadRequest, "invalid request body", "")
		return
	}
	if err := h.Svc.SetEnvironmentConnections(r.Context(), h.tenant(r), id, in.ConnectionIDs); err != nil {
		fail(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"environment_id": id, "connection_ids": in.ConnectionIDs})
}

func (h Handlers) listRules(w http.ResponseWriter, r *http.Request) {
	items, err := h.Svc.ListAllocationRules(r.Context(), h.tenant(r))
	if err != nil {
		fail(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h Handlers) createRule(w http.ResponseWriter, r *http.Request) {
	var in domain.AllocationRule
	if err := httpx.DecodeJSON(r, &in); err != nil {
		httpx.WriteProblem(w, r, http.StatusBadRequest, "invalid request body", "")
		return
	}
	in.ID = ""
	out, err := h.Svc.CreateAllocationRule(r.Context(), h.tenant(r), in)
	if err != nil {
		fail(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, out)
}

func (h Handlers) deleteRule(w http.ResponseWriter, r *http.Request) {
	id, ok := httpx.PathUUID(r, "id")
	if !ok {
		httpx.NotFound(w, r)
		return
	}
	if err := h.Svc.DeleteAllocationRule(r.Context(), h.tenant(r), id); err != nil {
		fail(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h Handlers) allocation(w http.ResponseWriter, r *http.Request) {
	from, to, err := httpx.Period(r)
	if err != nil {
		httpx.WriteProblem(w, r, http.StatusBadRequest, "invalid query", "")
		return
	}
	rep, err := h.Svc.Allocation(r.Context(), h.tenant(r), from, to)
	if err != nil {
		fail(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, rep)
}
