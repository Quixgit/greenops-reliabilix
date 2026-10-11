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
	"math"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/audit"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/auth"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/billing"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/database"
	httpx "github.com/quixgit/greenops-reliabilix/backend/internal/platform/http"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/notify"
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
	// Notify announces renewals and payments (the bell, the destinations and an e-mail copy); optional.
	Notify notify.Notifier
	// ReturnURL is where a payment provider sends the person afterwards (the web app's settings page).
	ReturnURL string
}

type Module struct{ d Deps }

func New(d Deps) *Module { return &Module{d: d} }

func (*Module) Name() string { return "licensing" }

func (m *Module) Routes(r chi.Router) {
	r.With(auth.Require(auth.PermProjectRead)).Get("/plan", m.plan)
	r.With(auth.Require(auth.PermBillingRead)).Get("/plan/history", m.history)
	r.With(auth.Require(auth.PermBillingRead)).Post("/plan/checkout", m.checkout)
}

// PublicRoutes: payment providers call back without a session; every notification is verified by its provider adapter.
func (m *Module) PublicRoutes(r chi.Router) {
	r.Post("/billing/webhooks/{provider}", m.webhook)
}

// RegisterJobs implements queue.JobRegistrar.
func (m *Module) RegisterJobs(mux *asynq.ServeMux) {
	mux.HandleFunc(queue.TaskBillingReminders, func(ctx context.Context, _ *asynq.Task) error {
		n, err := m.Remind(ctx, time.Now())
		if n > 0 {
			m.d.Log.Info("billing reminders sent", "organizations", n)
		}
		return err
	})
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
	// Catalog lists every plan, so the customer can compare them with the current one.
	Catalog []catalogRow `json:"catalog"`
}

type providerRow struct {
	ID    string   `json:"id"`
	Name  string   `json:"name"`
	Plans []string `json:"plans"`
	// Prices holds the amount per plan where the provider's price is known to the platform.
	Prices map[string]priceRow `json:"prices"`
}

type priceRow struct {
	Amount   float64 `json:"amount"`
	Currency string  `json:"currency"`
}

// catalogRow is one plan of the catalog as a customer sees it: what it includes.
type catalogRow struct {
	ID          string      `json:"id"`
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Rank        int         `json:"rank"` // position in the catalog: higher is a bigger plan
	Limits      []limitRow  `json:"limits"`
	Modules     []moduleRow `json:"modules"`
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
	v := planView{Status: "none", Source: "manual", Limits: []limitRow{}, Modules: []moduleRow{}, Providers: []providerRow{}, Catalog: []catalogRow{}}
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
				row := providerRow{ID: p.ID(), Name: p.Name(), Plans: p.Plans(r.Context()), Prices: map[string]priceRow{}}
				if pr, ok := p.(billing.Pricer); ok {
					for _, id := range row.Plans {
						if amount, cur, ok := pr.Price(r.Context(), id); ok {
							row.Prices[id] = priceRow{Amount: amount, Currency: cur}
						}
					}
				}
				v.Providers = append(v.Providers, row)
			}
		}
	}
	for i, p := range plans.Catalog() {
		row := catalogRow{ID: p.ID, Name: p.Name, Description: p.Description, Rank: i, Limits: []limitRow{}, Modules: []moduleRow{}}
		for _, u := range usage {
			row.Limits = append(row.Limits, limitRow{Resource: u.Resource, Key: u.Key, Limit: int64(p.Limits[u.Key])})
		}
		for _, k := range moduleKeys {
			row.Modules = append(row.Modules, moduleRow{Key: k, Enabled: p.Modules[k]})
		}
		v.Catalog = append(v.Catalog, row)
	}
	httpx.WriteJSON(w, http.StatusOK, v)
}

type historyRow struct {
	At       time.Time `json:"at"`
	Provider string    `json:"provider"`
	Kind     string    `json:"kind"` // started | requested | applied
	Plan     string    `json:"plan"`
	Detail   string    `json:"detail"`
}

// history lists the organization's own billing events: checkouts it started, requests an operator must handle and the
// payments a provider confirmed.
func (m *Module) history(w http.ResponseWriter, r *http.Request) {
	c, _ := auth.FromContext(r.Context())
	out := []historyRow{}
	err := database.WithTenantTx(r.Context(), m.d.Pool, c.TenantID, func(tx pgx.Tx) error {
		rows, err := tx.Query(r.Context(), `SELECT at, provider, kind, plan, detail FROM platform.tenant_billing_history(50)`)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var h historyRow
			if err := rows.Scan(&h.At, &h.Provider, &h.Kind, &h.Plan, &h.Detail); err != nil {
				return err
			}
			out = append(out, h)
		}
		return rows.Err()
	})
	if err != nil {
		httpx.Internal(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"items": out})
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
	// Which plan was asked for, and whether it renews, upgrades or lowers the current one, is kept in the organization's
	// own trail; the plan id also travels to the provider, so its confirmation names the plan that was paid for.
	intent := m.intent(r.Context(), c.TenantID, plan.ID)
	_ = database.WithTenantTx(r.Context(), m.d.Pool, c.TenantID, func(tx pgx.Tx) error {
		return audit.Record(r.Context(), tx, "billing.checkout_started", "plan:"+plan.ID, map[string]any{"provider": prov.ID(), "plan": plan.ID, "intent": intent})
	})
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"url": res.URL, "message": res.Message, "plan": plan.ID, "intent": intent})
}

// intent says what choosing a plan means for the organization: renew the current one, move up, or move down.
func (m *Module) intent(ctx context.Context, tenantID, target string) string {
	var cur string
	_ = database.WithTenantTx(ctx, m.d.Pool, tenantID, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx, `SELECT plan FROM tenants.organizations WHERE id = $1`, tenantID).Scan(&cur)
	})
	rank := func(id string) int {
		for i, p := range plans.Catalog() {
			if p.ID == id {
				return i
			}
		}
		return -1
	}
	switch {
	case cur == target:
		return "renew"
	case rank(target) > rank(cur):
		return "upgrade"
	default:
		return "downgrade"
	}
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
			if applied {
				m.paymentMessage(r.Context(), prov.ID(), ev.ID, ev.OrganizationID, plan, ev.Status, ev.PeriodEnd)
			}
		}
	}
	if ack, ok := prov.(billing.Acknowledger); ok {
		httpx.WriteJSON(w, http.StatusOK, ack.Ack(r.Context(), body))
		return
	}
	w.WriteHeader(http.StatusOK)
}

// announce publishes a billing event, with an e-mail copy to the organization's billing contacts when the organization
// wants it. It never fails the caller: money has been handled, the message is a courtesy.
func (m *Module) announce(ctx context.Context, tenantID string, e notify.Event, contacts []string) {
	if m.d.Notify == nil {
		return
	}
	var to []string
	if m.d.Settings != nil && m.d.Settings.Bool(ctx, tenantID, settings.KeyBillingEmailCopy) {
		to = contacts
	}
	if cp, ok := m.d.Notify.(notify.CopyPublisher); ok {
		if err := cp.PublishWithCopy(ctx, tenantID, e, to); err != nil {
			m.d.Log.Warn("billing message not sent", "tenant", tenantID, "type", e.Type, "err", err)
		}
		return
	}
	_ = m.d.Notify.Publish(ctx, tenantID, e)
}

// ReminderEvent builds the renewal message for a period that ends in daysLeft days. A period that has just ended
// gets one message saying so. The key repeats for the same period and the same distance, so a reminder goes once.
func ReminderEvent(planName string, periodEnd time.Time, daysLeft int) notify.Event {
	day := periodEnd.UTC().Format("2006-01-02")
	switch {
	case daysLeft < 0:
		return notify.Event{Type: notify.TypeBillingPlanEnded, Severity: notify.Warning, Title: planName + ": the paid period has ended",
			Body: "Renew the plan to keep its limits and modules. Existing data is kept.", Path: "/settings#plan", DedupeKey: "billing:ended:" + day}
	case daysLeft <= 1:
		return notify.Event{Type: notify.TypeBillingRenewalDue, Severity: notify.Critical, Title: planName + " ends tomorrow",
			Body: "The paid period ends on " + day + ". Renew now so nothing is interrupted.", Path: "/settings#plan", DedupeKey: "billing:renew:" + day + ":1"}
	case daysLeft <= 3:
		return notify.Event{Type: notify.TypeBillingRenewalDue, Severity: notify.Warning, Title: planName + " ends in " + itoa(daysLeft) + " days",
			Body: "The paid period ends on " + day + ". Renew to keep the plan.", Path: "/settings#plan", DedupeKey: "billing:renew:" + day + ":3"}
	default:
		return notify.Event{Type: notify.TypeBillingRenewalDue, Severity: notify.Info, Title: planName + " ends in " + itoa(daysLeft) + " days",
			Body: "The paid period ends on " + day + ". You can renew it on the Billing page.", Path: "/settings#plan", DedupeKey: "billing:renew:" + day + ":n"}
	}
}

func itoa(n int) string { return strconv.Itoa(n) }

// Remind sends the renewal reminders that are due and reports how many organizations were told something.
func (m *Module) Remind(ctx context.Context, now time.Time) (int, error) {
	rows, err := m.d.Pool.Query(ctx, `SELECT tenant_id::text, plan, status, period_end, emails FROM platform.billing_reminder_candidates($1)`, 30)
	if err != nil {
		return 0, err
	}
	type cand struct {
		tenant, plan, status string
		end                  time.Time
		emails               []string
	}
	var list []cand
	for rows.Next() {
		var c cand
		if err := rows.Scan(&c.tenant, &c.plan, &c.status, &c.end, &c.emails); err != nil {
			rows.Close()
			return 0, err
		}
		list = append(list, c)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, err
	}
	n := 0
	for _, c := range list {
		window := int(m.d.Settings.Int(ctx, c.tenant, settings.KeyBillingReminderDays))
		days := int(math.Ceil(c.end.Sub(now).Hours() / 24))
		if window <= 0 || (days > window && days >= 0) {
			continue
		}
		name := c.plan
		if p, ok := plans.Lookup(c.plan); ok {
			name = p.Name
		}
		m.announce(ctx, c.tenant, ReminderEvent(name, c.end, days), c.emails)
		n++
	}
	return n, nil
}

// paymentMessage tells the organization what the provider confirmed: a payment and the new period, a failed payment,
// or the end of the plan. The event id makes it once only.
func (m *Module) paymentMessage(ctx context.Context, provider, eventID, tenantID string, plan plans.Plan, status string, end *time.Time) {
	var contacts []string
	_ = m.d.Pool.QueryRow(ctx, `SELECT platform.tenant_billing_contacts($1::uuid)`, tenantID).Scan(&contacts)
	until := ""
	if end != nil {
		until = end.UTC().Format("2006-01-02")
	}
	e := notify.Event{Path: "/settings#plan", DedupeKey: "billing:event:" + provider + ":" + eventID, Attrs: map[string]string{"plan": plan.ID, "provider": provider}}
	switch {
	case status == "past_due":
		e.Type, e.Severity, e.Title = notify.TypeBillingPaymentFailed, notify.Warning, "A payment for "+plan.Name+" did not go through"
		e.Body = "Update the payment method on the Billing page to keep the plan."
	case status == "canceled" || plan.ID == "free":
		e.Type, e.Severity, e.Title = notify.TypeBillingPlanEnded, notify.Warning, "Your paid plan has ended"
		e.Body = "The organization is back on the Free plan. Existing data is kept."
	default:
		e.Type, e.Severity, e.Title = notify.TypeBillingPaymentReceived, notify.Info, "Payment received: "+plan.Name+" is active"
		e.Body = "Thank you."
		if until != "" {
			e.Body = "The plan is paid until " + until + ". Thank you."
		}
	}
	m.announce(ctx, tenantID, e, contacts)
}
