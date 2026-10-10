import { describe, expect, it } from "vitest";
import { changePct, computeSci, formatGrams } from "./sci";

describe("computeSci", () => {
  it("divides the period's carbon by the functional units", () => {
    const r = computeSci({ energyKwh: 100, carbonKg: 40, embodiedIncluded: false, units: 20_000 });
    expect(r?.perUnitG).toBeCloseTo(2, 10); // 40 000 g / 20 000
    expect(r?.operationalG).toBe(40_000);
    expect(r?.embodiedG).toBe(0);
    expect(r?.intensityGPerKwh).toBeCloseTo(400, 10);
  });
  it("does not invent a split when embodied emissions are part of the total", () => {
    const r = computeSci({ energyKwh: 10, carbonKg: 5, embodiedIncluded: true, units: 5 });
    expect(r?.perUnitG).toBeCloseTo(1000, 10);
    expect(Number.isNaN(r?.operationalG)).toBe(true);
  });
  it("refuses inputs that give no SCI", () => {
    for (const units of [0, -1, NaN, Infinity]) expect(computeSci({ energyKwh: 1, carbonKg: 1, embodiedIncluded: false, units })).toBeNull();
    expect(computeSci({ energyKwh: 1, carbonKg: -1, embodiedIncluded: false, units: 1 })).toBeNull();
  });
  it("handles a period without energy", () => {
    expect(computeSci({ energyKwh: 0, carbonKg: 0, embodiedIncluded: false, units: 10 })?.intensityGPerKwh).toBe(0);
  });
});

describe("changePct", () => {
  it("is relative to the base and absent without one", () => {
    expect(changePct(80, 100)).toBe(-20);
    expect(changePct(5, 0)).toBeNull();
  });
});

describe("formatGrams", () => {
  it("picks a readable unit", () => {
    expect(formatGrams(0.25)).toEqual({ value: 250, unit: "mg" });
    expect(formatGrams(12)).toEqual({ value: 12, unit: "g" });
    expect(formatGrams(2500)).toEqual({ value: 2.5, unit: "kg" });
    expect(formatGrams(0)).toEqual({ value: 0, unit: "g" });
  });
});
