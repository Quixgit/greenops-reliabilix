package domain

import (
	"context"
	"errors"
	"time"
)

// Intensity is grid carbon intensity at a point in time.
type Intensity struct {
	Region     string
	GPerKWh    float64
	At         time.Time
	IsForecast bool
}

// Forecast is a series of future intensities.
type Forecast struct {
	Region string
	Points []Intensity
}

// ErrNotConfigured is returned by a provider that has no credentials (yet). Callers fall back to stored readings
// quietly: it is a state of the deployment, not a failure of the provider.
var ErrNotConfigured = errors.New("carbon data provider is not configured")

// RegionLister is implemented by providers that can enumerate the regions they cover.
type RegionLister interface{ Regions() []string }

// ProviderElectricityMaps is the provider key stored in carbon.grid_intensity.
const ProviderElectricityMaps = "electricitymaps"

// CarbonDataProvider is implemented by Electricity Maps (now) and WattTime (phase 2).
type CarbonDataProvider interface {
	GetIntensity(ctx context.Context, region string) (Intensity, error)
	GetForecast(ctx context.Context, region string) (Forecast, error)
}
