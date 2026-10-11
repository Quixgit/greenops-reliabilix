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
	Measure    string // MeasureAverage (default when empty) or MeasureMarginal
}

// Forecast is a series of future intensities.
type Forecast struct {
	Region string
	Points []Intensity
}

// ErrNotConfigured is returned by a provider that has no credentials (yet). Callers fall back to stored readings
// quietly: it is a state of the deployment, not a failure of the provider.
var ErrNotConfigured = errors.New("carbon data provider is not configured")

// ErrPastUnavailable: the provider does not return past data on this key or plan. Not a failure of the job: other
// sources still fill the past, and the daily loading must not be retried over it.
var ErrPastUnavailable = errors.New("past data is not available on this key or plan")

// RegionLister is implemented by providers that can enumerate the regions they cover.
type RegionLister interface{ Regions() []string }

// Provider keys stored in carbon.grid_intensity. They equal the ids of the integrations catalog entries.
const (
	ProviderElectricityMaps = "electricitymaps"
	ProviderEmber           = "ember"
	ProviderEIA             = "eia"
	ProviderEnergyCharts    = "energy_charts"
	ProviderUKCI            = "uk_carbon_intensity"
	ProviderIESO            = "ieso"
	ProviderHydroQuebec     = "hydroquebec"
	ProviderENTSOE          = "entsoe"
	ProviderONS             = "ons"
	ProviderWattTime        = "watttime"
)

// CarbonDataProvider is implemented by Electricity Maps (now) and WattTime (phase 2).
type CarbonDataProvider interface {
	GetIntensity(ctx context.Context, region string) (Intensity, error)
	GetForecast(ctx context.Context, region string) (Forecast, error)
}

// HistoryProvider is implemented by providers that can return past hourly readings (a plan feature of the provider).
type HistoryProvider interface {
	GetHistory(ctx context.Context, region string, from, to time.Time) ([]Intensity, error)
}
