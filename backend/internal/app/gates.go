package app

import (
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/auth"
	httpx "github.com/quixgit/greenops-reliabilix/backend/internal/platform/http"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/settings"
)

// gatesModule registers request-level gates driven by settings (ADR-0023). It must be the first module: it adds
// middleware, and a router only accepts middleware before routes.
//   - maintenance mode: organization users can read but not change anything,
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
