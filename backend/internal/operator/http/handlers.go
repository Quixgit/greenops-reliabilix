// Package http exposes the operator console API under /api/v1/operator. Routes are mounted behind identity-only
// authentication (a valid user token, no tenant); each handler then requires an operator row in the database.
// API keys never reach this far: identity authentication rejects them.
package http

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/quixgit/greenops-reliabilix/backend/internal/operator/application"
	"github.com/quixgit/greenops-reliabilix/backend/internal/operator/domain"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/auth"
	httpx "github.com/quixgit/greenops-reliabilix/backend/internal/platform/http"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/integrations"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/methodology"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/queue"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/settings"
)

type Handlers struct{ Svc application.Service }

type opKey struct{}

func (h Handlers) IdentityRoutes(r chi.Router) {
	read, write := h.operator(false), h.operator(true)
	r.With(read).Get("/operator/me", h.me)
	r.With(read).Get("/operator/tenants", h.tenants)
	r.With(read).Get("/operator/notifications/stats", h.stats)
	r.With(read).Get("/operator/notifications/deliveries", h.deliveries)
	r.With(read).Get("/operator/connections/unhealthy", h.unhealthy)
	r.With(read).Get("/operator/system", h.system)
	r.With(read).Get("/operator/audit", h.audit)
	r.With(read).Get("/operator/data-sources", h.dataSources)
	r.With(read).Get("/operator/jobs", h.jobs)
	r.With(read).Get("/operator/operators", h.operators)
	r.With(read).Get("/operator/plans", h.plans)
	r.With(write).Post("/operator/carbon/backfill", h.backfill)
	r.With(write).Put("/operator/operators", h.assignOperator)
	r.With(write).Post("/operator/operators/withdraw", h.withdrawOperator)
	r.With(write).Put("/operator/tenants/{tenant_id}/plan", h.assignPlan)
	r.With(read).Get("/operator/tenants/{tenant_id}/subscription", h.subscription)
	r.With(read).Get("/operator/billing/events", h.billingEvents)
	r.With(read).Get("/operator/methodology", h.methodologyVersions)
	r.With(read).Get("/operator/api-usage", h.apiUsage)
	r.With(read).Get("/operator/errors", h.errors)
	r.With(read).Get("/operator/integrations", h.integrations)
	r.With(write).Put("/operator/integrations/{id}", h.setIntegration)
	r.With(write).Post("/operator/integrations/{id}/test", h.testIntegration)
	r.With(read).Get("/operator/settings", h.platformSettings)
	r.With(write).Patch("/operator/settings", h.setPlatformSettings)
	r.With(write).Post("/operator/settings/reset", h.resetPlatformSettings)
	r.With(read).Get("/operator/settings/history", h.settingsHistory)
	r.With(read).Get("/operator/tenants/{tenant_id}/settings", h.tenantSettings)
	r.With(write).Patch("/operator/tenants/{tenant_id}/settings", h.setTenantSettings)
	r.With(write).Post("/operator/tenants/{tenant_id}/settings/reset", h.resetTenantSettings)
	r.With(write).Post("/operator/errors/{id}/resolve", h.setErrorStatus("resolved"))
	r.With(write).Post("/operator/errors/{id}/ignore", h.setErrorStatus("ignored"))
	r.With(write).Post("/operator/errors/{id}/reopen", h.setErrorStatus("open"))
	r.With(write).Post("/operator/tenants/{tenant_id}/deliveries/{id}/retry", h.retry)
	r.With(write).Post("/operator/tenants/{tenant_id}/destinations/{id}/enable", h.setEnabled(true))
	r.With(write).Post("/operator/tenants/{tenant_id}/destinations/{id}/disable", h.setEnabled(false))
}

// operator authorizes the caller as a platform operator and logs the access. Anyone else gets a plain 403.
func (h Handlers) operator(write bool) httpx.Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			c, ok := auth.FromContext(r.Context())
			if !ok || c.Subject == "" {
				httpx.WriteProblem(w, r, http.StatusUnauthorized, "unauthorized", "")
				return
			}
			op, err := h.Svc.Authorize(r.Context(), c.Subject, c.Email, r.Method+" "+r.URL.Path, write)
			switch {
			case errors.Is(err, domain.ErrNotOperator), errors.Is(err, domain.ErrReadOnly):
				httpx.WriteProblem(w, r, http.StatusForbidden, "forbidden", "")
				return
			case err != nil:
				httpx.Internal(w, r, err)
				return
			}
			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), opKey{}, op)))
		})
	}
}

func operatorOf(r *http.Request) domain.Operator {
	op, _ := r.Context().Value(opKey{}).(domain.Operator)
	return op
}

func fail(w http.ResponseWriter, r *http.Request, err error) {
	var fe settings.FieldErrors
	switch {
	case errors.As(err, &fe):
		httpx.WriteValidation(w, r, "invalid settings", fe.FieldProblems())
	case errors.Is(err, settings.ErrNotFound):
		httpx.NotFound(w, r)
	case errors.Is(err, domain.ErrNotFound):
		httpx.NotFound(w, r)
	case errors.Is(err, domain.ErrRule):
		httpx.WriteProblem(w, r, http.StatusConflict, "not allowed", strings.TrimPrefix(err.Error(), domain.ErrRule.Error()+": "))
	case errors.Is(err, domain.ErrInvalid):
		httpx.WriteProblem(w, r, http.StatusBadRequest, "invalid query", "")
	default:
		httpx.Internal(w, r, err)
	}
}

func list[T any](w http.ResponseWriter, r *http.Request, items []T, err error) {
	if err != nil {
		fail(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h Handlers) me(w http.ResponseWriter, r *http.Request) {
	httpx.WriteJSON(w, http.StatusOK, operatorOf(r))
}

func (h Handlers) tenants(w http.ResponseWriter, r *http.Request) {
	items, err := h.Svc.Tenants(r.Context())
	list(w, r, items, err)
}

func (h Handlers) stats(w http.ResponseWriter, r *http.Request) {
	hours := 24
	if v := r.URL.Query().Get("hours"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			fail(w, r, domain.ErrInvalid)
			return
		}
		hours = n
	}
	items, err := h.Svc.DeliveryStats(r.Context(), hours)
	list(w, r, items, err)
}

func (h Handlers) deliveries(w http.ResponseWriter, r *http.Request) {
	limit := 50
	if v := r.URL.Query().Get("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 || n > 200 {
			httpx.WriteProblem(w, r, http.StatusBadRequest, "invalid query", "limit must be 1-200")
			return
		}
		limit = n
	}
	items, err := h.Svc.Deliveries(r.Context(), r.URL.Query().Get("status"), limit)
	list(w, r, items, err)
}

func (h Handlers) unhealthy(w http.ResponseWriter, r *http.Request) {
	items, err := h.Svc.UnhealthyConnections(r.Context())
	list(w, r, items, err)
}

func (h Handlers) system(w http.ResponseWriter, _ *http.Request) {
	httpx.WriteJSON(w, http.StatusOK, h.Svc.SystemInfo())
}

func (h Handlers) audit(w http.ResponseWriter, r *http.Request) {
	limit := 100
	if v := r.URL.Query().Get("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 || n > 500 {
			httpx.WriteProblem(w, r, http.StatusBadRequest, "invalid query", "limit must be 1-500")
			return
		}
		limit = n
	}
	items, err := h.Svc.Audit(r.Context(), limit)
	list(w, r, items, err)
}

func (h Handlers) dataSources(w http.ResponseWriter, r *http.Request) {
	items, err := h.Svc.DataSources(r.Context())
	list(w, r, items, err)
}

func (h Handlers) plans(w http.ResponseWriter, r *http.Request) {
	items, err := h.Svc.Plans(r.Context())
	list(w, r, items, err)
}

// assignPlan takes {"plan", "reason", "status"?, "period_end"?, "trial_ends_at"?} (dates as YYYY-MM-DD or RFC 3339).
func (h Handlers) assignPlan(w http.ResponseWriter, r *http.Request) {
	tenant, ok := httpx.PathUUID(r, "tenant_id")
	if !ok {
		httpx.NotFound(w, r)
		return
	}
	var in struct {
		Plan        string `json:"plan"`
		Reason      string `json:"reason"`
		Status      string `json:"status"`
		PeriodEnd   string `json:"period_end"`
		TrialEndsAt string `json:"trial_ends_at"`
	}
	if err := httpx.DecodeJSON(r, &in); err != nil {
		httpx.WriteProblem(w, r, http.StatusUnprocessableEntity, "invalid request", `body must be {"plan": "<id>", "reason": "<why>", "status"?, "period_end"?, "trial_ends_at"?}`)
		return
	}
	parse := func(v string) (*time.Time, bool) {
		if v == "" {
			return nil, true
		}
		for _, layout := range []string{time.RFC3339, time.DateOnly} {
			if t, err := time.Parse(layout, v); err == nil {
				t = t.UTC()
				return &t, true
			}
		}
		return nil, false
	}
	period, ok1 := parse(in.PeriodEnd)
	trial, ok2 := parse(in.TrialEndsAt)
	if !ok1 || !ok2 {
		httpx.WriteProblem(w, r, http.StatusUnprocessableEntity, "invalid request", "Dates are YYYY-MM-DD.")
		return
	}
	err := h.Svc.AssignPlan(r.Context(), operatorOf(r), tenant, application.PlanChange{Plan: in.Plan, Status: in.Status, TrialEnd: trial, PeriodEnd: period, Reason: in.Reason})
	if err != nil {
		if errors.Is(err, domain.ErrInvalid) || errors.Is(err, domain.ErrRule) {
			httpx.WriteProblem(w, r, http.StatusUnprocessableEntity, "invalid request", "Choose a plan of the catalog, a valid status (a trial needs an end date) and give a reason of 3 to 300 characters.")
			return
		}
		fail(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]string{"plan": in.Plan})
}

func (h Handlers) subscription(w http.ResponseWriter, r *http.Request) {
	tenant, ok := httpx.PathUUID(r, "tenant_id")
	if !ok {
		httpx.NotFound(w, r)
		return
	}
	s, err := h.Svc.Subscription(r.Context(), tenant)
	if err != nil {
		fail(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, s)
}

// apiUsage answers the requests made to paid external APIs per day (last 30 days) with the daily cap in force.
func (h Handlers) apiUsage(w http.ResponseWriter, r *http.Request) {
	items, err := h.Svc.Repo.APIUsage(r.Context(), 30)
	if err != nil {
		fail(w, r, err)
		return
	}
	caps := map[string]int64{}
	if h.Svc.Settings != nil {
		if its, err := h.Svc.Settings.PlatformItems(r.Context()); err == nil {
			for _, it := range its {
				if it.Def != nil && it.Key == settings.KeyCarbonMaxRequests {
					var n int64
					_ = json.Unmarshal(it.Value, &n)
					caps["electricitymaps"] = n
				}
			}
		}
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"items": items, "daily_caps": caps})
}

// methodologyVersions lists every coefficient set with its provenance, caveats, coefficients and checksum (a set is
// immutable; a test pins each checksum).
func (h Handlers) methodologyVersions(w http.ResponseWriter, r *http.Request) {
	type set struct {
		Version         string             `json:"version"`
		Status          string             `json:"status"`
		Provenance      string             `json:"provenance"`
		Notes           string             `json:"notes"`
		Caveats         []string           `json:"caveats"`
		PUE             float64            `json:"pue"`
		EnergyPerUnit   map[string]float64 `json:"energy_kwh_per_unit"`
		CostBasedKWhUSD map[string]float64 `json:"cost_based_kwh_per_usd"`
		EmbodiedGPerKWh float64            `json:"embodied_g_per_kwh"`
		Checksum        string             `json:"checksum"`
		Active          bool               `json:"active"`
	}
	out := []set{}
	for _, f := range methodology.Versions() {
		out = append(out, set{Version: f.Version, Status: f.Status, Provenance: f.Provenance, Notes: f.Notes, Caveats: f.Caveats, PUE: f.PUE,
			EnergyPerUnit: f.EnergyPerUnit, CostBasedKWhUSD: f.CostBasedKWhPerUSD, EmbodiedGPerKWh: f.EmbodiedGPerKWh,
			Checksum: methodology.Checksum(f.Version), Active: f.Version == methodology.Version})
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"items": out})
}

func (h Handlers) billingEvents(w http.ResponseWriter, r *http.Request) {
	items, err := h.Svc.BillingEvents(r.Context())
	list(w, r, items, err)
}

// assignOperator takes {"email": "...", "role": "viewer"|"admin"}: a person who has signed in becomes an operator, or
// their role changes.
func (h Handlers) assignOperator(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Email string      `json:"email"`
		Role  domain.Role `json:"role"`
	}
	if err := httpx.DecodeJSON(r, &in); err != nil {
		httpx.WriteProblem(w, r, http.StatusUnprocessableEntity, "invalid request", `body must be {"email": "...", "role": "viewer|admin"}`)
		return
	}
	if err := h.Svc.AssignOperator(r.Context(), operatorOf(r), in.Email, in.Role); err != nil {
		if errors.Is(err, domain.ErrInvalid) {
			httpx.WriteProblem(w, r, http.StatusUnprocessableEntity, "invalid request", "Give a valid e-mail address and the role viewer or admin.")
			return
		}
		fail(w, r, err)
		return
	}
	h.operators(w, r)
}

// withdrawOperator takes {"subject": "..."}.
func (h Handlers) withdrawOperator(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Subject string `json:"subject"`
	}
	if err := httpx.DecodeJSON(r, &in); err != nil {
		httpx.WriteProblem(w, r, http.StatusUnprocessableEntity, "invalid request", `body must be {"subject": "..."}`)
		return
	}
	if err := h.Svc.WithdrawOperator(r.Context(), operatorOf(r), in.Subject); err != nil {
		if errors.Is(err, domain.ErrInvalid) {
			httpx.WriteProblem(w, r, http.StatusUnprocessableEntity, "invalid request", "An operator subject is required.")
			return
		}
		fail(w, r, err)
		return
	}
	h.operators(w, r)
}

// backfill takes {"from": "YYYY-MM-DD", "to": "YYYY-MM-DD", "dry_run": bool}.
func (h Handlers) backfill(w http.ResponseWriter, r *http.Request) {
	var in struct {
		From   string `json:"from"`
		To     string `json:"to"`
		DryRun bool   `json:"dry_run"`
	}
	from, to := time.Time{}, time.Time{}
	err := httpx.DecodeJSON(r, &in)
	if err == nil {
		var e1, e2 error
		from, e1 = time.Parse(time.DateOnly, in.From)
		to, e2 = time.Parse(time.DateOnly, in.To)
		if e1 != nil || e2 != nil {
			err = domain.ErrInvalid
		}
	}
	if err != nil {
		httpx.WriteProblem(w, r, http.StatusUnprocessableEntity, "invalid request", `body must be {"from": "YYYY-MM-DD", "to": "YYYY-MM-DD", "dry_run": true|false}`)
		return
	}
	plan, err := h.Svc.BackfillGridHistory(r.Context(), from, to, in.DryRun)
	if errors.Is(err, domain.ErrInvalid) {
		httpx.WriteProblem(w, r, http.StatusUnprocessableEntity, "outside the limits", "Choose 1 to 400 days up to tomorrow and at most 2000 provider requests (regions x 9-day chunks).")
		return
	}
	if err != nil {
		fail(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, plan)
}

func (h Handlers) operators(w http.ResponseWriter, r *http.Request) {
	items, err := h.Svc.Operators(r.Context())
	list(w, r, items, err)
}

// jobs answers queues, schedules and failed tasks in one response: the Jobs page needs all three together.
func (h Handlers) jobs(w http.ResponseWriter, r *http.Request) {
	if h.Svc.Jobs == nil {
		httpx.NotFound(w, r)
		return
	}
	queues, err := h.Svc.Jobs.Queues()
	if err == nil {
		var failed []queue.FailedTask
		var schedules []queue.Schedule
		if failed, err = h.Svc.Jobs.Failed(50); err == nil {
			if schedules, err = h.Svc.Jobs.Schedules(); err == nil {
				httpx.WriteJSON(w, http.StatusOK, map[string]any{"queues": queues, "failed": failed, "schedules": schedules})
				return
			}
		}
	}
	httpx.WriteProblem(w, r, http.StatusServiceUnavailable, "queue unavailable", "The job queue could not be read.")
}

func ids(w http.ResponseWriter, r *http.Request) (tenant, id string, ok bool) {
	tenant, ok1 := httpx.PathUUID(r, "tenant_id")
	id, ok2 := httpx.PathUUID(r, "id")
	if !ok1 || !ok2 {
		httpx.NotFound(w, r)
		return "", "", false
	}
	return tenant, id, true
}

func (h Handlers) retry(w http.ResponseWriter, r *http.Request) {
	tenant, id, ok := ids(w, r)
	if !ok {
		return
	}
	if err := h.Svc.RetryDelivery(r.Context(), operatorOf(r), tenant, id); err != nil {
		fail(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusAccepted, map[string]bool{"queued": true})
}

func (h Handlers) setEnabled(enabled bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenant, id, ok := ids(w, r)
		if !ok {
			return
		}
		if err := h.Svc.SetDestinationEnabled(r.Context(), operatorOf(r), tenant, id, enabled); err != nil {
			fail(w, r, err)
			return
		}
		httpx.WriteJSON(w, http.StatusOK, map[string]bool{"enabled": enabled})
	}
}

func (h Handlers) errors(w http.ResponseWriter, r *http.Request) {
	status := r.URL.Query().Get("status")
	if status == "" {
		status = "open"
	}
	if status == "all" {
		status = ""
	}
	limit := 100
	if v := r.URL.Query().Get("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 || n > 200 {
			httpx.WriteProblem(w, r, http.StatusBadRequest, "invalid query", "limit must be 1-200")
			return
		}
		limit = n
	}
	items, err := h.Svc.Errors(r.Context(), status, r.URL.Query().Get("service"), limit)
	list(w, r, items, err)
}

func (h Handlers) setErrorStatus(status string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := httpx.PathUUID(r, "id")
		if !ok {
			httpx.NotFound(w, r)
			return
		}
		if err := h.Svc.SetErrorStatus(r.Context(), operatorOf(r), id, status); err != nil {
			fail(w, r, err)
			return
		}
		httpx.WriteJSON(w, http.StatusOK, map[string]string{"status": status})
	}
}

// ---- settings (ADR-0023) ----

type settingsView struct {
	Groups []settings.Group `json:"groups"`
	Items  []settings.Item  `json:"items"`
}

func (h Handlers) settingsOK(w http.ResponseWriter, r *http.Request) bool {
	if h.Svc.Settings == nil {
		httpx.WriteProblem(w, r, http.StatusServiceUnavailable, "settings unavailable", "")
		return false
	}
	return true
}

func (h Handlers) writeView(w http.ResponseWriter, r *http.Request, items []settings.Item, err error) {
	if err != nil {
		fail(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, settingsView{Groups: h.Svc.Settings.Groups(), Items: items})
}

func (h Handlers) integrations(w http.ResponseWriter, r *http.Request) {
	if h.Svc.Integrations == nil {
		httpx.NotFound(w, r)
		return
	}
	items, err := h.Svc.Integrations.Statuses(r.Context())
	list(w, r, items, err)
}

func integrationFail(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, integrations.ErrUnknown):
		httpx.NotFound(w, r)
	case errors.Is(err, integrations.ErrField):
		httpx.WriteProblem(w, r, http.StatusUnprocessableEntity, "invalid value", err.Error())
	case errors.Is(err, integrations.ErrNoBox):
		httpx.WriteProblem(w, r, http.StatusServiceUnavailable, "secrets cannot be stored", "No sealing key is configured on the server (NOTIFICATIONS_SECRET_KEY).")
	default:
		httpx.Internal(w, r, err)
	}
}

// setIntegration takes {"values": {"<field>": "<value>" | null}}. null returns the field to the deployment
// environment. Nothing is echoed back: the response is the new status without any secret.
func (h Handlers) setIntegration(w http.ResponseWriter, r *http.Request) {
	if h.Svc.Integrations == nil {
		httpx.NotFound(w, r)
		return
	}
	var in struct {
		Values map[string]*string `json:"values"`
	}
	if err := httpx.DecodeJSON(r, &in); err != nil || len(in.Values) == 0 || len(in.Values) > 20 {
		httpx.WriteProblem(w, r, http.StatusUnprocessableEntity, "invalid request", `body must be {"values": {"<field>": "<value>" | null}}`)
		return
	}
	id := chi.URLParam(r, "id")
	if err := h.Svc.Integrations.Set(r.Context(), operatorOf(r).Subject, id, in.Values); err != nil {
		integrationFail(w, r, err)
		return
	}
	h.integrations(w, r)
}

func (h Handlers) testIntegration(w http.ResponseWriter, r *http.Request) {
	if h.Svc.Integrations == nil {
		httpx.NotFound(w, r)
		return
	}
	msg, err := h.Svc.Integrations.Test(r.Context(), chi.URLParam(r, "id"))
	switch {
	case errors.Is(err, integrations.ErrUnknown):
		httpx.NotFound(w, r)
	case err != nil:
		httpx.WriteJSON(w, http.StatusOK, map[string]any{"ok": false, "message": err.Error()})
	default:
		httpx.WriteJSON(w, http.StatusOK, map[string]any{"ok": true, "message": msg})
	}
}

func decodeValues(w http.ResponseWriter, r *http.Request) (map[string]json.RawMessage, bool) {
	var in struct {
		Values map[string]json.RawMessage `json:"values"`
	}
	if err := httpx.DecodeJSON(r, &in); err != nil || len(in.Values) == 0 || len(in.Values) > 100 {
		httpx.WriteProblem(w, r, http.StatusUnprocessableEntity, "invalid request", `body must be {"values": {"<key>": <value>, ...}}`)
		return nil, false
	}
	return in.Values, true
}

func decodeKeys(w http.ResponseWriter, r *http.Request) ([]string, bool) {
	var in struct {
		Keys []string `json:"keys"`
	}
	if err := httpx.DecodeJSON(r, &in); err != nil || len(in.Keys) == 0 || len(in.Keys) > 100 {
		httpx.WriteProblem(w, r, http.StatusUnprocessableEntity, "invalid request", `body must be {"keys": ["<key>", ...]}`)
		return nil, false
	}
	return in.Keys, true
}

func (h Handlers) platformSettings(w http.ResponseWriter, r *http.Request) {
	if !h.settingsOK(w, r) {
		return
	}
	items, err := h.Svc.Settings.PlatformItems(r.Context())
	h.writeView(w, r, items, err)
}

func (h Handlers) setPlatformSettings(w http.ResponseWriter, r *http.Request) {
	values, ok := decodeValues(w, r)
	if !ok || !h.settingsOK(w, r) {
		return
	}
	if err := h.Svc.Settings.SetPlatform(r.Context(), operatorOf(r).Subject, values); err != nil {
		fail(w, r, err)
		return
	}
	h.platformSettings(w, r)
}

func (h Handlers) resetPlatformSettings(w http.ResponseWriter, r *http.Request) {
	keys, ok := decodeKeys(w, r)
	if !ok || !h.settingsOK(w, r) {
		return
	}
	if err := h.Svc.Settings.ResetPlatform(r.Context(), operatorOf(r).Subject, keys); err != nil {
		fail(w, r, err)
		return
	}
	h.platformSettings(w, r)
}

func (h Handlers) tenantSettings(w http.ResponseWriter, r *http.Request) {
	tenant, ok := httpx.PathUUID(r, "tenant_id")
	if !ok {
		httpx.NotFound(w, r)
		return
	}
	if !h.settingsOK(w, r) {
		return
	}
	items, err := h.Svc.Settings.OperatorTenantItems(r.Context(), tenant)
	h.writeView(w, r, items, err)
}

func (h Handlers) setTenantSettings(w http.ResponseWriter, r *http.Request) {
	tenant, ok := httpx.PathUUID(r, "tenant_id")
	if !ok {
		httpx.NotFound(w, r)
		return
	}
	values, ok := decodeValues(w, r)
	if !ok || !h.settingsOK(w, r) {
		return
	}
	if err := h.Svc.Settings.SetTenantOverride(r.Context(), operatorOf(r).Subject, tenant, values); err != nil {
		fail(w, r, err)
		return
	}
	h.tenantSettings(w, r)
}

func (h Handlers) resetTenantSettings(w http.ResponseWriter, r *http.Request) {
	tenant, ok := httpx.PathUUID(r, "tenant_id")
	if !ok {
		httpx.NotFound(w, r)
		return
	}
	keys, ok := decodeKeys(w, r)
	if !ok || !h.settingsOK(w, r) {
		return
	}
	if err := h.Svc.Settings.ClearTenantOverride(r.Context(), operatorOf(r).Subject, tenant, keys); err != nil {
		fail(w, r, err)
		return
	}
	h.tenantSettings(w, r)
}

func (h Handlers) settingsHistory(w http.ResponseWriter, r *http.Request) {
	if !h.settingsOK(w, r) {
		return
	}
	var tenant *string
	if v := r.URL.Query().Get("tenant_id"); v != "" {
		if !httpx.ParseUUID(v) {
			httpx.WriteProblem(w, r, http.StatusBadRequest, "invalid query", "tenant_id must be a UUID")
			return
		}
		tenant = &v
	}
	limit := 100
	if v := r.URL.Query().Get("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 || n > 500 {
			httpx.WriteProblem(w, r, http.StatusBadRequest, "invalid query", "limit must be 1-500")
			return
		}
		limit = n
	}
	items, err := h.Svc.Settings.OperatorHistory(r.Context(), tenant, limit)
	list(w, r, items, err)
}
