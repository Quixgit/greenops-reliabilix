// Package domain holds the Kubernetes energy rules: what a sample is, which samples are accepted and how a
// workload's carbon follows from its energy (ADR-0030). No database or HTTP code.
package domain

import (
	"context"
	"errors"
	"fmt"
	"math"
	"regexp"
	"time"
)

var ErrInvalidSample = errors.New("invalid sample")

var (
	namePattern   = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:/-]{0,126}$`)
	regionPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{1,38}$`)
	kinds         = map[string]bool{"deployment": true, "statefulset": true, "daemonset": true, "job": true, "cronjob": true, "pod": true, "other": true}
)

const (
	MaxSamplesPerRequest = 500
	// MaxKWhPerHour is the largest plausible energy of one workload in one hour (a very large GPU job): anything above
	// is a unit mistake (joules sent as kWh) and is refused instead of poisoning the totals.
	MaxKWhPerHour = 5000.0
	maxAge        = 400 * 24 * time.Hour
	// Bounds of the optional usage figures: an hour of 1024 cores, 64 TiB of memory. Beyond them is a unit mistake.
	maxCoreSeconds = 1024 * 3600.0
	maxBytes       = 64 * 1024 * 1024 * 1024 * 1024.0
	maxCores       = 4096.0
)

// Sample is one workload's measured energy in one UTC hour.
type Sample struct {
	Cluster     string    `json:"cluster"`
	Namespace   string    `json:"namespace"`
	Workload    string    `json:"workload"`
	Kind        string    `json:"kind"`
	PeriodStart time.Time `json:"period_start"`
	EnergyKWh   float64   `json:"energy_kwh"`
	Region      string    `json:"region"`
	// Usage and requests of the hour, summed over the workload's pods; all optional (an agent without cAdvisor and
	// kube-state-metrics sends energy only).
	CPUCoreSeconds     *float64 `json:"cpu_core_seconds,omitempty"`     // CPU time used in the hour
	MemoryAvgBytes     *float64 `json:"memory_avg_bytes,omitempty"`     // average working set
	CPURequestCores    *float64 `json:"cpu_request_cores,omitempty"`    // average requested cores
	MemoryRequestBytes *float64 `json:"memory_request_bytes,omitempty"` // average requested memory
}

// Validate refuses what cannot be a real measurement. now is injected for tests.
func (s Sample) Validate(now time.Time) error {
	switch {
	case !namePattern.MatchString(s.Cluster), !namePattern.MatchString(s.Namespace), !namePattern.MatchString(s.Workload):
		return fmt.Errorf("%w: cluster, namespace and workload must be 1-127 characters of letters, digits and . _ : / -", ErrInvalidSample)
	case !kinds[s.Kind]:
		return fmt.Errorf("%w: unknown kind %q", ErrInvalidSample, s.Kind)
	case !regionPattern.MatchString(s.Region):
		return fmt.Errorf("%w: region must be a cloud region id such as eu-central-1", ErrInvalidSample)
	case math.IsNaN(s.EnergyKWh) || math.IsInf(s.EnergyKWh, 0) || s.EnergyKWh < 0 || s.EnergyKWh > MaxKWhPerHour:
		return fmt.Errorf("%w: energy_kwh must be between 0 and %.0f per hour (kWh, not joules)", ErrInvalidSample, MaxKWhPerHour)
	case !s.PeriodStart.Equal(s.PeriodStart.UTC().Truncate(time.Hour)):
		return fmt.Errorf("%w: period_start must be the start of a UTC hour", ErrInvalidSample)
	case s.PeriodStart.Add(time.Hour).After(now.UTC()):
		return fmt.Errorf("%w: period_start is in the future or the hour is not over", ErrInvalidSample)
	case s.PeriodStart.Before(now.Add(-maxAge)):
		return fmt.Errorf("%w: period_start is older than 400 days", ErrInvalidSample)
	}
	for name, v := range map[string]struct {
		val *float64
		max float64
	}{
		"cpu_core_seconds": {s.CPUCoreSeconds, maxCoreSeconds}, "memory_avg_bytes": {s.MemoryAvgBytes, maxBytes},
		"cpu_request_cores": {s.CPURequestCores, maxCores}, "memory_request_bytes": {s.MemoryRequestBytes, maxBytes},
	} {
		if v.val != nil && (math.IsNaN(*v.val) || math.IsInf(*v.val, 0) || *v.val < 0 || *v.val > v.max) {
			return fmt.Errorf("%w: %s is out of range", ErrInvalidSample, name)
		}
	}
	return nil
}

// Carbon returns the emissions of a measured energy: kWh x PUE x intensity. ok is false when no grid reading exists,
// in which case the energy is still kept and the carbon stays unknown (never guessed).
func Carbon(kwh, pue, gPerKWh float64, haveIntensity bool) (kg float64, ok bool) {
	if !haveIntensity {
		return 0, false
	}
	return kwh * pue * gPerKWh / 1000, true
}

// WorkloadRow is a workload's energy over a window.
type WorkloadRow struct {
	Cluster   string   `json:"cluster"`
	Namespace string   `json:"namespace"`
	Workload  string   `json:"workload"`
	Kind      string   `json:"kind"`
	EnergyKWh float64  `json:"energy_kwh"`
	CarbonKg  *float64 `json:"carbon_kg_co2e"` // nil when some hours had no grid data: a partial sum is never shown as a total
	Hours     int      `json:"hours"`
	NoGrid    int      `json:"hours_without_grid_data"`
	// Utilization: usage divided by what the workload requested, over the hours that have both. Nil without such hours.
	CPUUtilizationPct    *float64 `json:"cpu_utilization_pct"`
	MemoryUtilizationPct *float64 `json:"memory_utilization_pct"`
	UsageHours           int      `json:"usage_hours"`
}

// ClusterRow is a cluster's energy over a window.
type ClusterRow struct {
	Cluster    string   `json:"cluster"`
	Workloads  int      `json:"workloads"`
	Namespaces int      `json:"namespaces"`
	EnergyKWh  float64  `json:"energy_kwh"`
	CarbonKg   *float64 `json:"carbon_kg_co2e"`
	LastHour   string   `json:"last_hour"`
}

// IntensitySource is implemented in the composition root from the carbon module.
type IntensitySource interface {
	IntensityAt(ctx context.Context, region string, at time.Time) (g float64, ok bool, err error)
}

// Repository is tenant-scoped (explicit tenant id and RLS).
type Repository interface {
	ProjectExists(ctx context.Context, tenantID, projectID string) (bool, error)
	Upsert(ctx context.Context, tenantID, projectID string, rows []Stored) error
	Workloads(ctx context.Context, tenantID string, project *string, cluster string, from, to time.Time, limit int) ([]WorkloadRow, error)
	Clusters(ctx context.Context, tenantID string, project *string, from, to time.Time) ([]ClusterRow, error)
}

// Utilization is usage / request in percent; ok is false when there is no request to compare with.
func Utilization(used, requested float64) (pct float64, ok bool) {
	if requested <= 0 {
		return 0, false
	}
	return used / requested * 100, true
}

// Stored is a sample with its grid reading and carbon, as persisted.
type Stored struct {
	Sample
	Intensity          *float64
	CarbonKg           *float64
	MethodologyVersion string
}
