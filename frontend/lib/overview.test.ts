import { describe, expect, it } from "vitest";
import { monthForecast, movingAverage, rollup, share } from "./overview";

const day = (i: number, cost: number, kg: number) => ({ day: `2026-10-${String(i).padStart(2, "0")}T00:00:00Z`, cost, carbon_kg_co2e: kg });

describe("rollup", () => {
  it("averages per day and compares the last week with the one before", () => {
    const pts = Array.from({ length: 14 }, (_, i) => day(i + 1, i < 7 ? 10 : 15, i < 7 ? 1 : 2));
    const r = rollup(pts);
    expect(r.cost).toBe(7 * 10 + 7 * 15);
    expect(r.costPerDay).toBeCloseTo(12.5);
    expect(r.costTrend7).toBeCloseTo(50);
    expect(r.carbonTrend7).toBeCloseTo(100);
    expect(r.gramsPerCurrency).toBeCloseTo((21 * 1000) / 175);
  });
  it("does not compare with fewer than 14 days", () => {
    expect(rollup([day(1, 5, 1), day(2, 5, 1)]).costTrend7).toBeNull();
    expect(rollup([]).costPerDay).toBe(0);
    expect(rollup([]).gramsPerCurrency).toBeNull();
  });
});

describe("monthForecast", () => {
  const now = new Date("2026-10-11T12:00:00Z");
  it("adds the run-rate of the last 7 days for the remaining days", () => {
    const pts = Array.from({ length: 10 }, (_, i) => day(i + 1, 10, 1));
    const f = monthForecast(pts, now)!;
    expect(f.spent).toBe(100);
    expect(f.daysLeft).toBe(21);
    expect(f.forecast).toBeCloseTo(100 + 10 * 21);
  });
  it("is null when the month has no data", () => {
    expect(monthForecast([{ day: "2026-09-10T00:00:00Z", cost: 1, carbon_kg_co2e: 1 }], now)).toBeNull();
  });
});

describe("helpers", () => {
  it("smooths and shares", () => {
    expect(movingAverage([1, 3, 5], 2)).toEqual([1, 2, 4]);
    expect(share(25, 100)).toBe(25);
    expect(share(1, 0)).toBe(0);
  });
});
