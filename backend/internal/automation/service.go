// Package automation is the phase 3 module (Terraform/Kubernetes execution with approval).
package automation

import "github.com/go-chi/chi/v5"

type Module struct{}

func New() *Module { return &Module{} }

func (*Module) Name() string        { return "automation" }
func (*Module) Routes(r chi.Router) {}
