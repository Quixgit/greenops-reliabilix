// Gateway is the single public entry point: it authenticates the caller, then
// reverse-proxies to internal services. Services re-verify the token themselves
// (zero trust), so a bypassed gateway does not bypass authorization.
package main

import (
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"

	"github.com/quixgit/greenops-reliabilix/pkg/config"
	"github.com/quixgit/greenops-reliabilix/pkg/service"
)

// route maps a public path prefix to an upstream env var and default.
var routes = []struct{ prefix, env, def string }{
	{"/v1/me", "UPSTREAM_TENANT", "http://localhost:8081"},
	{"/v1/projects", "UPSTREAM_PROJECT", "http://localhost:8082"},
	{"/v1/cloud-accounts", "UPSTREAM_CLOUD", "http://localhost:8083"},
	{"/v1/usage", "UPSTREAM_INGESTION", "http://localhost:8084"},
	{"/v1/carbon", "UPSTREAM_CARBON", "http://localhost:8085"},
}

func main() {
	service.Run("gateway", "8080", service.Options{}, func(mux *http.ServeMux, d service.Deps) {
		for _, r := range routes {
			target, err := url.Parse(config.String(r.env, r.def))
			if err != nil {
				log.Fatalf("bad upstream %s: %v", r.env, err)
			}
			proxy := httputil.NewSingleHostReverseProxy(target)
			proxy.ErrorHandler = func(w http.ResponseWriter, req *http.Request, err error) {
				d.Log.Error("upstream", "err", err, "path", req.URL.Path)
				w.WriteHeader(http.StatusBadGateway)
			}
			mux.Handle(r.prefix, proxy)
			mux.Handle(r.prefix+"/", proxy)
		}
	})
}
