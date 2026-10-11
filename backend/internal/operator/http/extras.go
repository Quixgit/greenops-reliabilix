package http

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	httpx "github.com/quixgit/greenops-reliabilix/backend/internal/platform/http"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/notify"
)

// extraRoutes are the console's cross-organization views and announcements. They read and write through database
// functions that check the operator's role themselves; the handlers only authorize and shape the answer.
func (h Handlers) extraRoutes(r chi.Router) {
	read, write := h.operator(false), h.operator(true)
	r.With(read).Get("/operator/metrics", h.metrics)
	r.With(read).Get("/operator/announcements", h.announcements)
	r.With(write).Post("/operator/announcements", h.announce)
	r.With(write).Post("/operator/announcements/{id}/withdraw", h.withdrawAnnouncement)
	r.With(read).Get("/operator/tenants/{tenant_id}/overview", h.orgOverview)
	r.With(write).Post("/operator/tenants/{tenant_id}/erasure", h.requestErasure)
	r.With(write).Post("/operator/tenants/{tenant_id}/erasure/cancel", h.cancelErasure)
}

func (h Handlers) metrics(w http.ResponseWriter, r *http.Request) {
	days := 30
	if v := r.URL.Query().Get("days"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 7 || n > 180 {
			httpx.WriteProblem(w, r, http.StatusBadRequest, "invalid query", "days must be 7-180")
			return
		}
		days = n
	}
	var raw []byte
	if err := h.Pool.QueryRow(r.Context(), `SELECT platform.operator_metrics($1)::text`, days).Scan(&raw); err != nil {
		fail(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(raw)
}

type announcementRow struct {
	ID            string     `json:"id"`
	Kind          string     `json:"kind"`
	Severity      string     `json:"severity"`
	Title         string     `json:"title"`
	Body          string     `json:"body"`
	Path          string     `json:"path"`
	AudiencePlans []string   `json:"audience_plans"`
	EmailCopy     bool       `json:"email_copy"`
	CreatedBy     string     `json:"created_by"`
	CreatedAt     time.Time  `json:"created_at"`
	Recipients    int        `json:"recipients"`
	WithdrawnAt   *time.Time `json:"withdrawn_at"`
	ReadBy        int        `json:"read_by"`
}

func (h Handlers) announcements(w http.ResponseWriter, r *http.Request) {
	rows, err := h.Pool.Query(r.Context(), `SELECT id::text, kind, severity, title, body, path, audience_plans, email_copy, created_by, created_at, recipients, withdrawn_at, read_by::int
		FROM platform.operator_announcements(100)`)
	if err != nil {
		fail(w, r, err)
		return
	}
	defer rows.Close()
	out := []announcementRow{}
	for rows.Next() {
		var a announcementRow
		if err := rows.Scan(&a.ID, &a.Kind, &a.Severity, &a.Title, &a.Body, &a.Path, &a.AudiencePlans, &a.EmailCopy, &a.CreatedBy, &a.CreatedAt, &a.Recipients, &a.WithdrawnAt, &a.ReadBy); err != nil {
			fail(w, r, err)
			return
		}
		out = append(out, a)
	}
	list(w, r, out, rows.Err())
}

func (h Handlers) announce(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Kind          string   `json:"kind"`
		Severity      string   `json:"severity"`
		Title         string   `json:"title"`
		Body          string   `json:"body"`
		Path          string   `json:"path"`
		AudiencePlans []string `json:"audience_plans"`
		EmailCopy     bool     `json:"email_copy"`
	}
	if err := httpx.DecodeJSON(r, &in); err != nil {
		httpx.WriteProblem(w, r, http.StatusBadRequest, "invalid request body", "")
		return
	}
	okKind := in.Kind == "update" || in.Kind == "maintenance" || in.Kind == "notice" || in.Kind == "incident"
	okSev := in.Severity == "info" || in.Severity == "warning" || in.Severity == "critical"
	if !okKind || !okSev || len(in.Title) < 3 || len(in.Title) > 200 || len(in.Body) > 2000 || len(in.Path) > 200 || (in.Path != "" && in.Path[0] != '/') {
		httpx.WriteProblem(w, r, http.StatusUnprocessableEntity, "invalid request", "kind update|maintenance|notice|incident, severity info|warning|critical, a title of 3-200 characters, a body up to 2000, an app-relative path.")
		return
	}
	if in.AudiencePlans == nil {
		in.AudiencePlans = []string{}
	}
	op := operatorOf(r)
	var id string
	var n int
	if err := h.Pool.QueryRow(r.Context(), `SELECT id::text, recipients FROM platform.operator_announce($1,$2,$3,$4,$5,$6,$7,$8)`,
		op.Subject, in.Kind, in.Severity, in.Title, in.Body, in.Path, in.AudiencePlans, in.EmailCopy).Scan(&id, &n); err != nil {
		fail(w, r, err)
		return
	}
	mailed := 0
	if in.EmailCopy && h.Mail != nil {
		rows, err := h.Pool.Query(r.Context(), `SELECT tenant_id::text, emails FROM platform.announcement_contacts($1, $2)`, op.Subject, in.AudiencePlans)
		if err == nil {
			type target struct {
				tenant string
				emails []string
			}
			var ts []target
			for rows.Next() {
				var t target
				if rows.Scan(&t.tenant, &t.emails) == nil {
					ts = append(ts, t)
				}
			}
			rows.Close()
			sev := notify.Severity(in.Severity)
			for _, t := range ts {
				if len(t.emails) == 0 {
					continue
				}
				e := notify.Event{Type: notify.TypePlatformAnnouncement, Severity: sev, Title: in.Title, Body: in.Body, Path: in.Path}
				if h.Mail.MailCopy(r.Context(), t.tenant, e, t.emails) == nil {
					mailed++
				}
			}
		}
	}
	httpx.WriteJSON(w, http.StatusCreated, map[string]any{"id": id, "recipients": n, "mailed_organizations": mailed})
}

func (h Handlers) withdrawAnnouncement(w http.ResponseWriter, r *http.Request) {
	id, ok := httpx.PathUUID(r, "id")
	if !ok {
		httpx.NotFound(w, r)
		return
	}
	var n int
	if err := h.Pool.QueryRow(r.Context(), `SELECT platform.operator_withdraw_announcement($1, $2::uuid)`, operatorOf(r).Subject, id).Scan(&n); err != nil {
		fail(w, r, err)
		return
	}
	if n < 0 {
		httpx.NotFound(w, r)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"removed": n})
}

type timelineRow struct {
	At     time.Time `json:"at"`
	Actor  string    `json:"actor"`
	Action string    `json:"action"`
	Target string    `json:"target"`
}

func (h Handlers) orgOverview(w http.ResponseWriter, r *http.Request) {
	tenant, ok := httpx.PathUUID(r, "tenant_id")
	if !ok {
		httpx.NotFound(w, r)
		return
	}
	var raw []byte
	if err := h.Pool.QueryRow(r.Context(), `SELECT coalesce(platform.operator_org_overview($1::uuid)::text, 'null')`, tenant).Scan(&raw); err != nil {
		fail(w, r, err)
		return
	}
	if string(raw) == "null" {
		httpx.NotFound(w, r)
		return
	}
	rows, err := h.Pool.Query(r.Context(), `SELECT at, actor, action, target FROM platform.operator_org_timeline($1::uuid, 100)`, tenant)
	if err != nil {
		fail(w, r, err)
		return
	}
	defer rows.Close()
	tl := []timelineRow{}
	for rows.Next() {
		var t timelineRow
		if err := rows.Scan(&t.At, &t.Actor, &t.Action, &t.Target); err != nil {
			fail(w, r, err)
			return
		}
		tl = append(tl, t)
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"overview": json.RawMessage(raw), "timeline": tl})
}

func (h Handlers) requestErasure(w http.ResponseWriter, r *http.Request) {
	tenant, ok := httpx.PathUUID(r, "tenant_id")
	if !ok {
		httpx.NotFound(w, r)
		return
	}
	var in struct {
		GraceDays int    `json:"grace_days"`
		Reason    string `json:"reason"`
	}
	if err := httpx.DecodeJSON(r, &in); err != nil || in.GraceDays < 1 || in.GraceDays > 90 || len(in.Reason) < 5 || len(in.Reason) > 300 {
		httpx.WriteProblem(w, r, http.StatusUnprocessableEntity, "invalid request", "grace_days 1-90 and a reason of 5-300 characters.")
		return
	}
	var done bool
	if err := h.Pool.QueryRow(r.Context(), `SELECT platform.operator_request_erasure($1,$2::uuid,$3,$4)`, operatorOf(r).Subject, tenant, in.GraceDays, in.Reason).Scan(&done); err != nil {
		fail(w, r, err)
		return
	}
	if !done {
		httpx.NotFound(w, r)
		return
	}
	httpx.WriteJSON(w, http.StatusAccepted, map[string]any{"scheduled": true, "grace_days": in.GraceDays})
}

func (h Handlers) cancelErasure(w http.ResponseWriter, r *http.Request) {
	tenant, ok := httpx.PathUUID(r, "tenant_id")
	if !ok {
		httpx.NotFound(w, r)
		return
	}
	var in struct {
		Reason string `json:"reason"`
	}
	_ = httpx.DecodeJSON(r, &in)
	var done bool
	if err := h.Pool.QueryRow(r.Context(), `SELECT platform.operator_cancel_erasure($1,$2::uuid,$3)`, operatorOf(r).Subject, tenant, in.Reason).Scan(&done); err != nil {
		fail(w, r, err)
		return
	}
	if !done {
		httpx.NotFound(w, r)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"cancelled": true})
}
