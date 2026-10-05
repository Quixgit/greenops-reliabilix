// Package domain holds the Project aggregate.
package domain

import (
	"context"
	"errors"
	"strings"
	"time"
)

type Project struct {
	ID        string    `json:"id"`
	TenantID  string    `json:"tenant_id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
}

var (
	ErrInvalidName = errors.New("name must be 1-120 characters")
	ErrDuplicate   = errors.New("project already exists")
)

func NewProject(id, tenantID, name string, now time.Time) (Project, error) {
	name = strings.TrimSpace(name)
	if n := len([]rune(name)); n < 1 || n > 120 {
		return Project{}, ErrInvalidName
	}
	return Project{ID: id, TenantID: tenantID, Name: name, CreatedAt: now}, nil
}

// Repository is tenant-scoped by construction: every method takes tenantID, and
// the Postgres implementation additionally relies on RLS (pkg/db.WithTenantTx).
type Repository interface {
	List(ctx context.Context, tenantID string) ([]Project, error)
	Create(ctx context.Context, p Project) error
}
