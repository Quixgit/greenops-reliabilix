import { describe, expect, it } from "vitest";
import { cleanWorld, splitRing } from "./geo";

describe("splitRing", () => {
  it("leaves a ring that does not cross the antimeridian untouched", () => {
    const ring: [number, number][] = [[0, 0], [10, 0], [10, 10], [0, 10], [0, 0]];
    expect(splitRing(ring)).toEqual([ring]);
  });

  it("cuts a ring that crosses +-180 into parts that each stay on their own side", () => {
    // A box from 170 east across the antimeridian to -170 (the data jumps from 180 to -180).
    const ring: [number, number][] = [[170, 60], [180, 60], [-180, 60], [-170, 60], [-170, 70], [-180, 70], [180, 70], [170, 70], [170, 60]];
    const parts = splitRing(ring);
    expect(parts.length).toBe(2);
    for (const part of parts) {
      const xs = part.map((p) => p[0]);
      expect(Math.max(...xs) - Math.min(...xs)).toBeLessThanOrEqual(10.0001); // no part spans the world any more
      expect(xs.every((x) => x >= -180.0001 && x <= 180.0001)).toBe(true);
    }
    const xs = parts.flat().map((p) => p[0]);
    expect(Math.max(...xs)).toBeCloseTo(180); expect(Math.min(...xs)).toBeCloseTo(-180);
  });
});

describe("cleanWorld", () => {
  it("drops Antarctica and repairs crossing polygons", () => {
    const fc = { features: [
      { type: "Feature" as const, properties: { name: "Antarctica" }, geometry: { type: "Polygon", coordinates: [[[0, -80], [10, -80], [10, -70], [0, -80]]] } },
      { type: "Feature" as const, properties: { name: "Russia" }, geometry: { type: "Polygon", coordinates: [[[170, 60], [180, 60], [-180, 60], [-170, 60], [-170, 70], [-180, 70], [180, 70], [170, 70], [170, 60]]] } },
      { type: "Feature" as const, properties: { name: "Spain" }, geometry: { type: "Polygon", coordinates: [[[0, 40], [5, 40], [5, 43], [0, 40]]] } },
    ] };
    const out = cleanWorld(fc);
    expect(out.features.map((f) => f.properties?.name)).toEqual(["Russia", "Spain"]);
    expect(out.features[0].geometry.type).toBe("MultiPolygon");
    expect(out.features[1].geometry.type).toBe("Polygon");
  });
});
