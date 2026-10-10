package app

import (
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/auth"
	httpx "github.com/quixgit/greenops-reliabilix/backend/internal/platform/http"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/i18n"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/settings"
)

// gatesModule registers request-level gates driven by settings (ADR-0023). It must be the first module: it adds
// middleware, and a router only accepts middleware before routes.
//   - maintenance mode: organization users can read but not change anything,
//   - suspended organization: the same, for that organization only, set by an operator,
//   - module switches: a disabled module answers 403 for that organization,
//   - sign-up policy: onboarding can be closed or limited to e-mail domains.
type gatesModule struct{ cfg settings.Reader }

func (gatesModule) Name() string { return "gates" }

// modulePrefixes maps URL prefixes (below /api/v1) to the module switch that controls them.
var modulePrefixes = []struct{ prefix, key, name string }{
	{"/automation", settings.KeyModuleAutomation, "Automation"},
	{"/kubernetes", settings.KeyModuleKubernetes, "Kubernetes"},
	{"/reports", settings.KeyModuleReports, "Reports"},
	{"/recommendations", settings.KeyModuleRecommendations, "Recommendations"},
	{"/notifications", settings.KeyModuleNotifications, "Notifications"},
	{"/ci/", settings.KeyModuleCIGate, "CI gate"},
}

func isRead(m string) bool {
	return m == http.MethodGet || m == http.MethodHead || m == http.MethodOptions
}

func (g gatesModule) Routes(r chi.Router) {
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			c, ok := auth.FromContext(req.Context())
			if !ok || c.TenantID == "" {
				next.ServeHTTP(w, req)
				return
			}
			path := strings.TrimPrefix(req.URL.Path, "/api/v1")
			// CI evaluation only reads: it keeps working during maintenance.
			if !isRead(req.Method) && path != "/ci/evaluate" && g.cfg.Bool(req.Context(), "", settings.KeyMaintenanceEnabled) {
				w.Header().Set("Retry-After", "300")
				httpx.WriteProblem(w, req, http.StatusServiceUnavailable, "maintenance", g.cfg.String(req.Context(), "", settings.KeyMaintenanceMessage))
				return
			}
			// A suspended organization keeps read access (including exports) and loses every change.
			if !isRead(req.Method) && g.cfg.Bool(req.Context(), c.TenantID, settings.KeyOrgSuspended) {
				httpx.WriteProblem(w, req, http.StatusForbidden, "organization suspended", g.cfg.String(req.Context(), c.TenantID, settings.KeyOrgSuspendedMessage))
				return
			}
			for _, m := range modulePrefixes {
				if (path == m.prefix || strings.HasPrefix(path, strings.TrimSuffix(m.prefix, "/")+"/")) && !g.cfg.Bool(req.Context(), c.TenantID, m.key) {
					httpx.WriteProblem(w, req, http.StatusForbidden, "module disabled", m.name+" is not enabled for your organization.")
					return
				}
			}
			next.ServeHTTP(w, req)
		})
	})
}

// PublicRoutes serves the language configuration without a session: the languages on offer and the default one, from
// the platform settings (i18n.enabled_locales, i18n.default_locale), limited to languages this build has a catalog for.
func (g gatesModule) PublicRoutes(r chi.Router) {
	r.Get("/locales", func(w http.ResponseWriter, req *http.Request) {
		enabled := []string{}
		for _, l := range g.cfg.Strings(req.Context(), "", settings.KeyEnabledLocales) {
			if i18n.Has(l) {
				enabled = append(enabled, l)
			}
		}
		def := g.cfg.String(req.Context(), "", settings.KeyDefaultLocale)
		if !i18n.Has(def) {
			def = i18n.Default
		}
		if len(enabled) == 0 {
			enabled = []string{def}
		}
		w.Header().Set("Cache-Control", "public, max-age=60")
		httpx.WriteJSON(w, http.StatusOK, map[string]any{"default": def, "enabled": enabled, "supported": i18n.Supported})
	})
}

// IdentityRoutes adds the sign-up policy in front of onboarding (the only route that creates organizations).
func (g gatesModule) IdentityRoutes(r chi.Router) {
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			if req.Method == http.MethodPost && strings.HasSuffix(req.URL.Path, "/onboarding") {
				if !g.cfg.Bool(req.Context(), "", settings.KeySignupsEnabled) {
					httpx.WriteProblem(w, req, http.StatusForbidden, "sign-up closed", "New organizations cannot be created right now. Ask for an invitation.")
					return
				}
				if allowed := g.cfg.Strings(req.Context(), "", settings.KeySignupsAllowedDomains); len(allowed) > 0 {
					c, _ := auth.FromContext(req.Context())
					if !emailInDomains(c.Email, allowed) {
						httpx.WriteProblem(w, req, http.StatusForbidden, "sign-up restricted", "Your e-mail domain cannot create an organization. Ask for an invitation.")
						return
					}
				}
			}
			next.ServeHTTP(w, req)
		})
	})
}

func emailInDomains(email string, domains []string) bool {
	at := strings.LastIndex(email, "@")
	if at < 1 {
		return false
	}
	d := strings.ToLower(email[at+1:])
	for _, a := range domains {
		if d == a {
			return true
		}
	}
	return false
}
