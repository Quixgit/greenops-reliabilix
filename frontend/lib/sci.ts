/** Software Carbon Intensity (Green Software Foundation): SCI = ((E x I) + M) / R, in gCO2e per functional unit R. */
export interface SciInput {
  /** E: energy of the period in kWh. */
  energyKwh: number;
  /** Total carbon of the period in kg CO2e (operational, plus embodied when the methodology includes it). */
  carbonKg: number;
  /** Whether the active methodology includes embodied emissions (M). */
  embodiedIncluded: boolean;
  /** R: functional units delivered in the period. */
  units: number;
}

export interface SciResult {
  /** SCI in gCO2e per unit. */
  perUnitG: number;
  /** E x I in grams (NaN when the total includes M and the split is unknown). */
  operationalG: number;
  /** M in grams (0 when the methodology does not include it, NaN when included but not reported separately). */
  embodiedG: number;
  /** I implied by the period: grams per kWh (0 when there is no energy). */
  intensityGPerKwh: number;
}

/** Returns null when the inputs cannot give a SCI: R must be positive and the period must have data. */
export function computeSci(i: SciInput): SciResult | null {
  if (!Number.isFinite(i.units) || i.units <= 0) return null;
  if (!Number.isFinite(i.carbonKg) || i.carbonKg < 0 || !Number.isFinite(i.energyKwh) || i.energyKwh < 0) return null;
  const totalG = i.carbonKg * 1000;
  const intensity = i.energyKwh > 0 ? totalG / i.energyKwh : 0;
  // The period total is E x I + M. The summary does not report M separately, so when the methodology includes it
  // the split is not shown (the total and the per-unit figure are still exact); without it everything is E x I.
  return { perUnitG: totalG / i.units, operationalG: i.embodiedIncluded ? NaN : totalG, embodiedG: i.embodiedIncluded ? NaN : 0, intensityGPerKwh: intensity };
}

/** Relative change of `now` against `before` in percent, or null when there is no base. */
export function changePct(now: number, before: number): number | null {
  if (!Number.isFinite(before) || before <= 0) return null;
  return ((now - before) / before) * 100;
}

/** Grams with a sensible unit: mg below 1 g, kg from 1000 g. */
export function formatGrams(g: number): { value: number; unit: "mg" | "g" | "kg" } {
  if (g >= 1000) return { value: g / 1000, unit: "kg" };
  if (g >= 1 || g === 0) return { value: g, unit: "g" };
  return { value: g * 1000, unit: "mg" };
}
