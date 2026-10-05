// Package recommendations merges finops and carbon findings (phase 2).
package recommendations

import "github.com/go-chi/chi/v5"

type Module struct{}

func New() *Module { return &Module{} }

func (*Module) Name() string        { return "recommendations" }
func (*Module) Routes(r chi.Router) {}
