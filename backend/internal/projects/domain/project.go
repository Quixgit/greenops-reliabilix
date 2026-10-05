// Package domain holds the Project aggregate and its rules (no DB/HTTP imports).
package domain

import (
	"context"
	"errors"
	"strings"
	"time"
)

type Project struct {
	ID             string    `json:"id"`
	TenantID       string    `json:"tenant_id"`
	Name           string    `json:"name"`
	FunctionalUnit string    `json:"functional_unit"` // SCI "R", e.g. "api_request", "active_user"
	CreatedAt      time.Time `json:"created_at"`
}

var (
	ErrInvalidName = errors.New("name must be 1-120 characters")
	ErrInvalidUnit = errors.New("functional_unit must be 1-60 characters")
	ErrDuplicate   = errors.New("project already exists")
)

func New(tenantID, name, functionalUnit string, now time.Time) (Project, error) {
	name, functionalUnit = strings.TrimSpace(name), strings.TrimSpace(functionalUnit)
	if n := len([]rune(name)); n < 1 || n > 120 {
		return Project{}, ErrInvalidName
	}
	if n := len([]rune(functionalUnit)); n < 1 || n > 60 {
		return Project{}, ErrInvalidUnit
	}
	return Project{TenantID: tenantID, Name: name, FunctionalUnit: functionalUnit, CreatedAt: now}, nil
}

// Repository is tenant-scoped by construction (explicit tenantID) and backed by RLS.
type Repository interface {
	List(ctx context.Context, tenantID string) ([]Project, error)
	Create(ctx context.Context, p Project) (Project, error)
}
