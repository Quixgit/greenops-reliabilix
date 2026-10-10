// Package licensing is the organization's view of its plan and the subscription clock (ADR-0022, ADR-0038). Limits and
// module switches are enforced by the settings engine and the gates that already exist; this module only reports them
// with their usage, and returns an organization to the Free plan when its period has ended.
package licensing

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/auth"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/billing"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/database"
	httpx "github.com/quixgit/greenops-reliabilix/backend/internal/platform/http"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/plans"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/queue"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/settings"
)

type Deps struct {
	Pool     *pgxpool.Pool
	Settings settings.Reader
	Log      *slog.Logger
	// Billing is the payment providers (optional: without it the plan page has no way to buy).
	Billing *billing.Registry
	// ReturnURL is where a payment provider sends the person afterwards (the web app's settings page).
	ReturnURL string
}

type Module struct{ d Deps }

func New(d Deps) *Module { return &Module{d: d} }

func (*Module) Name() string { return "licensing" }

func (m *Module) Routes(r chi.Router) {
	r.With(auth.Require(auth.PermProjectRead)).Get("/plan", m.plan)
	r.With(auth.Require(auth.PermTenantAdmin)).Post("/plan/checkout", m.checkout)
}

// PublicRoutes: payment providers call back without a session; every notification is verified by its provider adapter.
func (m *Module) PublicRoutes(r chi.Router) {
	r.Post("/billing/webhooks/{provider}", m.webhook)
}

// RegisterJobs implements queue.JobRegistrar.
func (m *Module) RegisterJobs(mux *asynq.ServeMux) {
	mux.HandleFunc(queue.TaskExpireSubscriptions, func(ctx context.Context, _ *asynq.Task) error {
		n, err := m.Expire(ctx)
		if n > 0 {
			m.d.Log.Info("subscriptions expired", "organizations", n)
		}
		return err
	})
}

// Expire returns organizations whose period has ended to the Free plan and reports how many changed.
func (m *Module) Expire(ctx context.Context) (n int, err error) {
	free, ok := plans.Lookup("free")
	if !ok {
		return 0, nil
	}
	values := map[string]any{}
	for k, raw := range free.Values() {
		values[k] = raw
	}
	err = m.d.Pool.QueryRow(ctx, `SELECT platform.expire_subscriptions($1::jsonb)`, mustJSON(values)).Scan(&n)
	return
}

type limitRow struct {
	Resource string `json:"resource"`
	Key      string `json:"key"`
	Limit    int64  `json:"limit"` // 0 = unlimited
	Used     int64  `json:"used"`
}

type moduleRow struct {
	Key     string `json:"key"`
	Enabled bool   `json:"enabled"`
}

// planView is what an organization sees about its plan.
type planView struct {
	Plan        string      `json:"plan"`
	Name        string      `json:"name"`
	Status      string      `json:"status"` // active | trialing | past_due | canceled | none (no subscription recorded)
	TrialEndsAt *time.Time  `json:"trial_ends_at"`
	PeriodEnd   *time.Time  `json:"period_end"`
	Source      string      `json:"source"`
	Limits      []limitRow  `json:"limits"`
	Modules     []moduleRow `json:"modules"`
	// Providers are the payment providers that are set up, with the plans each can sell.
	Providers []providerRow `json:"providers"`
}

type providerRow struct {
	ID    string   `json:"id"`
	Name  string   `json:"name"`
	Plans []string `json:"plans"`
}

var usage = []struct {
	Resource, Key, SQL string
}{
	{"projects", settings.KeyLimitProjects, `SELECT count(*) FROM projects.projects`},
	{"cloud_connections", settings.KeyLimitConnections, `SELECT count(*) FROM cloudaccounts.connections`},
	{"members", settings.KeyLimitMembers, `SELECT (SELECT count(*) FROM tenants.memberships) + (SELECT count(*) FROM tenants.invitations WHERE accepted_at IS NULL AND expires_at > now())`},
	{"notification_destinations", settings.KeyLimitDestinations, `SELECT count(*) FROM notifications.destinations`},
}

var moduleKeys = []string{settings.KeyModuleAutomation, settings.KeyModuleKubernetes, settings.KeyModuleReports, settings.KeyModuleRecommendations, settings.KeyModuleNotifications, settings.KeyModuleCIGate}

func (m *Module) plan(w http.ResponseWriter, r *http.Request) {
	c, _ := auth.FromContext(r.Context())
	v := planView{Status: "none", Source: "manual", Limits: []limitRow{}, Modules: []moduleRow{}, Providers: []providerRow{}}
	err := database.WithTenantTx(r.Context(), m.d.Pool, c.TenantID, func(tx pgx.Tx) error {
		if err := tx.QueryRow(r.Context(), `SELECT plan FROM tenants.organizations WHERE id = $1`, c.TenantID).Scan(&v.Plan); err != nil {
			return err
		}
		var status, source string
		var trial, period *time.Time
		err := tx.QueryRow(r.Context(), `SELECT status, source, trial_ends_at, period_end FROM tenants.subscriptions WHERE organization_id = $1`, c.TenantID).Scan(&status, &source, &trial, &period)
		switch {
		case err == nil:
			v.Status, v.Source, v.TrialEndsAt, v.PeriodEnd = status, source, trial, period
		case !database.IsNotFound(err):
			return err
		}
		for _, u := range usage {
			row := limitRow{Resource: u.Resource, Key: u.Key, Limit: m.d.Settings.Int(r.Context(), c.TenantID, u.Key)}
			if err := tx.QueryRow(r.Context(), u.SQL).Scan(&row.Used); err != nil {
				return err
			}
			v.Limits = append(v.Limits, row)
		}
		return nil
	})
	if err != nil {
		httpx.Internal(w, r, err)
		return
	}
	v.Name = v.Plan
	if p, ok := plans.Lookup(v.Plan); ok {
		v.Name = p.Name
	}
	for _, k := range moduleKeys {
		v.Modules = append(v.Modules, moduleRow{Key: k, Enabled: m.d.Settings.Bool(r.Context(), c.TenantID, k)})
	}
	if m.d.Billing != nil {
		for _, p := range m.d.Billing.All() {
			if p.Configured(r.Context()) {
				v.Providers = append(v.Providers, providerRow{ID: p.ID(), Name: p.Name(), Plans: p.Plans(r.Context())})
			}
		}
	}
	httpx.WriteJSON(w, http.StatusOK, v)
}

func mustJSON(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}

// checkout starts a payment for the organization at the chosen provider and answers where to send the person.
func (m *Module) checkout(w http.ResponseWriter, r *http.Request) {
	c, _ := auth.FromContext(r.Context())
	var in struct {
		Provider string `json:"provider"`
		Plan     string `json:"plan"`
	}
	if err := httpx.DecodeJSON(r, &in); err != nil || m.d.Billing == nil {
		httpx.WriteProblem(w, r, http.StatusBadRequest, "invalid request body", "")
		return
	}
	prov, ok := m.d.Billing.Get(in.Provider)
	plan, okPlan := plans.Lookup(in.Plan)
	if !ok || !okPlan || plan.ID == "free" || !prov.Configured(r.Context()) {
		httpx.WriteProblem(w, r, http.StatusUnprocessableEntity, "invalid request", "Choose a payment provider that is set up and a paid plan it sells.")
		return
	}
	offered := false
	for _, p := range prov.Plans(r.Context()) {
		offered = offered || p == plan.ID
	}
	if !offered {
		httpx.WriteProblem(w, r, http.StatusUnprocessableEntity, "invalid request", "This provider does not sell that plan.")
		return
	}
	var name string
	if err := m.d.Pool.QueryRow(r.Context(), `SELECT name FROM tenants.organizations WHERE id = $1`, c.TenantID).Scan(&name); err != nil {
		name = ""
	}
	res, err := prov.Checkout(r.Context(), billing.CheckoutRequest{OrganizationID: c.TenantID, OrganizationName: name, Email: c.Email, Plan: plan.ID, ReturnURL: m.d.ReturnURL})
	switch {
	case errors.Is(err, billing.ErrNotConfigured), errors.Is(err, billing.ErrUnknownPlan):
		httpx.WriteProblem(w, r, http.StatusUnprocessableEntity, "invalid request", err.Error())
		return
	case err != nil:
		m.d.Log.Warn("billing checkout failed", "provider", in.Provider, "err", err)
		httpx.WriteProblem(w, r, http.StatusBadGateway, "payment provider unavailable", "The payment provider did not accept the request. Try again later.")
		return
	}
	if mp, manual := prov.(billing.Manual); manual && mp.Manual() { // the platform will not hear of the payment: the operator must hear of the request
		if _, err := m.d.Pool.Exec(r.Context(), `SELECT platform.billing_request($1::uuid, $2, $3, $4)`, c.TenantID, prov.ID(), plan.ID, c.Email); err != nil {
			m.d.Log.Error("billing request not recorded", "err", err)
		}
	}
	httpx.WriteJSON(w, http.StatusOK, res)
}

// webhook applies a verified provider notification. The answer is 200 for everything that was understood (applied,
// repeated or ignored) so the provider stops retrying, 400 for an unverifiable one, 503 when the provider is not set up.
func (m *Module) webhook(w http.ResponseWriter, r *http.Request) {
	if m.d.Billing == nil {
		httpx.NotFound(w, r)
		return
	}
	prov, ok := m.d.Billing.Get(chi.URLParam(r, "provider"))
	if !ok {
		httpx.NotFound(w, r)
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		httpx.WriteProblem(w, r, http.StatusBadRequest, "invalid request body", "")
		return
	}
	ev, err := prov.ParseWebhook(r.Context(), r, body)
	switch {
	case errors.Is(err, billing.ErrNotConfigured):
		httpx.WriteProblem(w, r, http.StatusServiceUnavailable, "payment provider not set up", "")
		return
	case errors.Is(err, billing.ErrBadSignature):
		httpx.WriteProblem(w, r, http.StatusBadRequest, "invalid request body", "")
		return
	case err != nil:
		httpx.WriteProblem(w, r, http.StatusBadGateway, "payment provider unavailable", "")
		return
	}
	if ev != nil {
		if plan, ok := plans.Lookup(ev.Plan); ok {
			var end any
			if ev.PeriodEnd != nil {
				end = *ev.PeriodEnd
			}
			var applied bool
			err := m.d.Pool.QueryRow(r.Context(), `SELECT platform.billing_apply($1, $2, $3::uuid, $4, $5, $6, $7::jsonb, $8)`,
				prov.ID(), ev.ID, ev.OrganizationID, plan.ID, ev.Status, end, mustJSON(plan.Values()), ev.Reference).Scan(&applied)
			if err != nil {
				m.d.Log.Error("billing event not applied", "provider", prov.ID(), "event", ev.ID, "err", err)
				httpx.WriteProblem(w, r, http.StatusInternalServerError, "internal error", "")
				return // not acknowledged: the provider delivers it again
			}
		}
	}
	if ack, ok := prov.(billing.Acknowledger); ok {
		httpx.WriteJSON(w, http.StatusOK, ack.Ack(r.Context(), body))
		return
	}
	w.WriteHeader(http.StatusOK)
}
