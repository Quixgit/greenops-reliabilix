// Package infrastructure contains adapters. The in-memory repository is for
// tests/skeleton only; the PostgreSQL adapter replaces it (sqlc + pkg/db).
package infrastructure

import (
	"context"
	"sync"

	"github.com/quixgit/greenops-reliabilix/services/project/internal/domain"
)

type MemoryRepo struct {
	mu   sync.RWMutex
	data map[string][]domain.Project // keyed by tenant
}

func NewMemoryRepo() *MemoryRepo { return &MemoryRepo{data: map[string][]domain.Project{}} }

func (r *MemoryRepo) List(_ context.Context, tenantID string) ([]domain.Project, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return append([]domain.Project{}, r.data[tenantID]...), nil
}

func (r *MemoryRepo) Create(_ context.Context, p domain.Project) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, e := range r.data[p.TenantID] {
		if e.Name == p.Name {
			return domain.ErrDuplicate
		}
	}
	r.data[p.TenantID] = append(r.data[p.TenantID], p)
	return nil
}
