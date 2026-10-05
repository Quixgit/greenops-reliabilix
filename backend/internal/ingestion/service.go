// Package ingestion normalizes raw provider data into UsageRecords (phase 1 core path:
// cloudaccounts -> ingestion -> usage -> carbon). TODO: Cost Explorer normalizer + persist + enqueue carbon:calculate.
package ingestion

import "github.com/go-chi/chi/v5"

type Module struct{}

func New() *Module { return &Module{} }

func (*Module) Name() string        { return "ingestion" }
func (*Module) Routes(r chi.Router) {}
