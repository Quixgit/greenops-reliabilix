// Package reports is a phase 2 module; the layer directories are in place.
package reports

import "github.com/go-chi/chi/v5"

type Module struct{}

func New() *Module { return &Module{} }

func (*Module) Name() string        { return "reports" }
func (*Module) Routes(r chi.Router) {}
