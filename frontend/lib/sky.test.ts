import { describe, expect, it } from "vitest";
import { localHour, skyAt } from "./sky";

describe("skyAt", () => {
  it("is dark at night and bright at noon", () => {
    expect(skyAt(2).night).toBe(1);
    expect(skyAt(12).night).toBe(0);
  });
  it("changes smoothly through the evening", () => {
    const v = [17.5, 18.5, 19.5, 20.5].map((h) => skyAt(h).night);
    expect(v[0]).toBe(0);
    expect(v[3]).toBe(1);
    expect(v[1]).toBeGreaterThan(v[0]);
    expect(v[2]).toBeGreaterThan(v[1]);
  });
  it("glows at the horizon, morning and evening", () => {
    expect(skyAt(6.2).glow).toBeGreaterThan(0.9);
    expect(skyAt(6.2).morning).toBe(true);
    expect(skyAt(19).morning).toBe(false);
    expect(skyAt(12).glow).toBeLessThan(0.01);
  });
});

describe("localHour", () => {
  it("takes a preview override", () => { expect(localHour(new Date(), "21:30")).toBe(21.5); });
  it("ignores nonsense", () => { expect(localHour(new Date(2026, 0, 1, 9, 30), "x")).toBe(9.5); });
});
