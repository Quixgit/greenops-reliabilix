package httpx

import (
	"net/http"
	"regexp"

	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/database"
)

var cloudRe = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,31}$`)

// CloudScope narrows a read to one cloud when the request carries ?cloud=aws (or azure, gcp, ...): the Overview shows one cloud at a time, in
// that cloud's own currency and by its own arithmetic, instead of a sum over clouds billed in different currencies. Only GET requests are
// scoped; a malformed value is ignored (the request is then unscoped, never partially scoped).
func CloudScope() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodGet {
				if c := r.URL.Query().Get("cloud"); c != "" && cloudRe.MatchString(c) {
					r = r.WithContext(database.WithCloudScope(r.Context(), c))
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}
