package app

import (
	"context"
	"time"

	carbonapp "github.com/quixgit/greenops-reliabilix/backend/internal/carbon/application"
	k8sapp "github.com/quixgit/greenops-reliabilix/backend/internal/kubernetes/application"
	usageapp "github.com/quixgit/greenops-reliabilix/backend/internal/usage/application"
)

// carbonIntensity gives the Kubernetes module the stored grid reading of a region at a time, without importing carbon.
type carbonIntensity struct{ carbon *carbonapp.Service }

func (c carbonIntensity) IntensityAt(ctx context.Context, region string, at time.Time) (float64, bool, error) {
	return c.carbon.StoredIntensityAt(ctx, region, at)
}

// clusterPool gives the Kubernetes module the cost of a cluster's nodes from the cloud bill, without importing usage.
type clusterPool struct{ usage usageapp.Service }

func (c clusterPool) DailyPool(ctx context.Context, tenantID, projectID string, services []string, from, to time.Time) ([]k8sapp.PoolDay, string, error) {
	days, cur, err := c.usage.DailyPool(ctx, tenantID, projectID, services, from, to)
	out := make([]k8sapp.PoolDay, 0, len(days))
	for _, d := range days {
		out = append(out, k8sapp.PoolDay{Day: d.Day, Cost: d.Cost})
	}
	return out, cur, err
}

func (c clusterPool) Services(ctx context.Context, tenantID, projectID string, from, to time.Time) ([]k8sapp.PoolService, string, error) {
	items, cur, err := c.usage.PoolServices(ctx, tenantID, projectID, from, to)
	out := make([]k8sapp.PoolService, 0, len(items))
	for _, i := range items {
		out = append(out, k8sapp.PoolService{Name: i.Name, Cost: i.Cost})
	}
	return out, cur, err
}
