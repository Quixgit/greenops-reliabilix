package domain

import "errors"

// SCI computes the Software Carbon Intensity per functional unit:
// SCI = ((E * I) + M) / R, in gCO2e per R.
//
//	energyKWh: E, intensityGPerKWh: I, embodiedG: M (gCO2e), units: R (> 0).
func SCI(energyKWh, intensityGPerKWh, embodiedG, units float64) (float64, error) {
	if units <= 0 {
		return 0, errors.New("sci: functional units must be > 0")
	}
	if energyKWh < 0 || intensityGPerKWh < 0 || embodiedG < 0 {
		return 0, ErrInvalidInput
	}
	return (energyKWh*intensityGPerKWh + embodiedG) / units, nil
}
