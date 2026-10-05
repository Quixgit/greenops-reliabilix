package httpx

import "github.com/go-chi/chi/v5"

// Module is the unit of modularity: every business domain exposes exactly one
// Module from its package root (service.go). cmd/api mounts them all; a domain
// extracted into its own service later keeps this same shape.
type Module interface {
	Name() string
	// Routes registers the domain's endpoints under /api/v1. Authentication is
	// already applied; handlers add auth.Require(<permission>) per route.
	Routes(r chi.Router)
}
