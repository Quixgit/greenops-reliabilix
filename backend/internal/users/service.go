// Package users handles user profiles and tenant roles. Authentication itself
// is delegated to Auth0 (ADR-0004).
package users

import (
	"github.com/go-chi/chi/v5"

	uhttp "github.com/quixgit/greenops-reliabilix/backend/internal/users/http"
)

type Module struct{}

func New() *Module { return &Module{} }

func (*Module) Name() string        { return "users" }
func (*Module) Routes(r chi.Router) { uhttp.Handlers{}.Routes(r) }
