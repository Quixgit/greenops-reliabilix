package app

import (
	"context"
	"time"

	carbonapp "github.com/quixgit/greenops-reliabilix/backend/internal/carbon/application"
)

// carbonIntensity gives the Kubernetes module the stored grid reading of a region at a time, without importing carbon.
type carbonIntensity struct{ carbon *carbonapp.Service }

func (c carbonIntensity) IntensityAt(ctx context.Context, region string, at time.Time) (float64, bool, error) {
	return c.carbon.StoredIntensityAt(ctx, region, at)
}
