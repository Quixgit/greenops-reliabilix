import { describe, expect, it } from "vitest";
import { latencyBetween } from "./regions";

describe("latencyBetween", () => {
  it("uses a published measurement where one exists", () => {
    const l = latencyBetween("us-east-1", "us-west-2");
    expect(l?.basis).toBe("measured_aws");
    expect(l?.ms).toBeGreaterThan(60);
    expect(l?.errorPct).toBe(0);
  });
  it("is the same in both directions", () => {
    expect(latencyBetween("westeurope", "eastus")?.ms).toBe(latencyBetween("eastus", "westeurope")?.ms);
  });
  it("estimates between clouds and says how far off it may be", () => {
    const l = latencyBetween("us-east-1", "europe-west4");
    expect(l?.basis).toBe("estimated");
    expect(l?.errorPct).toBeGreaterThan(0);
  });
  it("is unknown for a region not on the map", () => {
    expect(latencyBetween("us-east-1", "nowhere-1")).toBeNull();
  });
});
