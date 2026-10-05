// Package domain contains pure carbon-accounting rules. It must not import databases, HTTP or SDKs
// (enforced by depguard in .golangci.yml).
package domain

import (
	"errors"
	"fmt"
	"strings"

	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/methodology"
)

// Current is the active coefficient set and MethodologyVersion the version stored with every calculation
// (both live in platform/methodology so that every reader of calculations agrees on the version).
var (
	Current            = methodology.Current
	MethodologyVersion = methodology.Version
)

// UsageKind classifies measured usage.
type UsageKind string

const (
	KindVCPUHours   UsageKind = "vcpu_hours"
	KindMemoryGBHrs UsageKind = "memory_gb_hours"
	KindStorageGBMo UsageKind = "storage_gb_months"
)

// Method says how the energy figure was obtained; it is stored with each calculation.
type Method string

const (
	UsageBased Method = "usage_based" // from measured consumption (higher confidence)
	CostBased  Method = "cost_based"  // estimated from spend (low confidence)
)

var ErrInvalidInput = errors.New("invalid carbon input")

// Input is one usage measurement plus the grid intensity of its region.
type Input struct {
	Kind                 UsageKind
	Amount               float64 // in Kind's unit, must be >= 0
	GridIntensityGPerKWh float64 // gCO2e per kWh, must be >= 0
}

// Result is the outcome of a calculation.
type Result struct {
	EnergyKWh          float64 `json:"energy_kwh"`
	CO2eKg             float64 `json:"co2e_kg"`
	MethodologyVersion string  `json:"methodology_version"`
}

// EnergyKWh converts measured usage into energy including PUE.
func EnergyKWh(kind UsageKind, amount float64) (float64, error) {
	coef, ok := Current.EnergyPerUnit[string(kind)]
	if !ok {
		return 0, fmt.Errorf("%w: unknown usage kind %q", ErrInvalidInput, kind)
	}
	if amount < 0 {
		return 0, fmt.Errorf("%w: negative amount", ErrInvalidInput)
	}
	return amount * coef * Current.PUE, nil
}

// Calculate converts usage to energy and operational emissions.
func Calculate(in Input) (Result, error) {
	if in.GridIntensityGPerKWh < 0 {
		return Result{}, fmt.Errorf("%w: negative grid intensity", ErrInvalidInput)
	}
	kwh, err := EnergyKWh(in.Kind, in.Amount)
	if err != nil {
		return Result{}, err
	}
	return Result{EnergyKWh: kwh, CO2eKg: kwh * in.GridIntensityGPerKWh / 1000, MethodologyVersion: MethodologyVersion}, nil
}

// CostBasedEnergyKWh estimates energy from spend for a FOCUS service category. Credits (negative
// cost) never produce negative energy.
func CostBasedEnergyKWh(category string, usd float64) (float64, bool) {
	coef, ok := Current.CostBasedKWhPerUSD[category]
	if !ok {
		coef, ok = Current.CostBasedKWhPerUSD["other"]
	}
	if !ok {
		return 0, false
	}
	if usd < 0 {
		usd = 0
	}
	return usd * coef * Current.PUE, true
}

// KindForUnit maps a FOCUS ConsumedUnit string to a measurable usage kind.
func KindForUnit(unit string) (UsageKind, bool) {
	switch strings.ToLower(strings.TrimSpace(unit)) {
	case "vcpu-hrs", "vcpu-hours", "vcpu_hours", "vcpuhours":
		return KindVCPUHours, true
	case "gb-hours", "gb-hrs", "gb_hours", "memory_gb_hours":
		return KindMemoryGBHrs, true
	case "gb-month", "gb-mo", "gb_months", "gb-months", "storage_gb_months":
		return KindStorageGBMo, true
	}
	return "", false
}
