// Package domain contains pure carbon-accounting rules. It must not import
// databases, HTTP or SDKs (enforced by depguard in .golangci.yml).
package domain

import (
	"errors"
	"fmt"
)

// MethodologyVersion is stored with every calculation so historic numbers stay
// reproducible when factors or formulas change (ADR-0007).
const MethodologyVersion = "CCF-2026.1"

// UsageKind classifies measured usage.
type UsageKind string

const (
	KindVCPUHours   UsageKind = "vcpu_hours"
	KindMemoryGBHrs UsageKind = "memory_gb_hours"
	KindStorageGBMo UsageKind = "storage_gb_months"
)

// Energy coefficients, kWh per unit. Placeholders: real values come from the
// versioned Cloud Carbon Footprint coefficient tables loaded by the factors module.
var energyPerUnit = map[UsageKind]float64{
	KindVCPUHours:   0.0021,
	KindMemoryGBHrs: 0.000392,
	KindStorageGBMo: 0.00065 * 730,
}

// pue is the data-centre power usage effectiveness assumed by the methodology.
const pue = 1.135

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

var ErrInvalidInput = errors.New("invalid carbon input")

// Calculate converts usage to energy and operational emissions.
func Calculate(in Input) (Result, error) {
	coef, ok := energyPerUnit[in.Kind]
	if !ok {
		return Result{}, fmt.Errorf("%w: unknown usage kind %q", ErrInvalidInput, in.Kind)
	}
	if in.Amount < 0 || in.GridIntensityGPerKWh < 0 {
		return Result{}, fmt.Errorf("%w: negative value", ErrInvalidInput)
	}
	kwh := in.Amount * coef * pue
	return Result{
		EnergyKWh:          kwh,
		CO2eKg:             kwh * in.GridIntensityGPerKWh / 1000,
		MethodologyVersion: MethodologyVersion,
	}, nil
}
