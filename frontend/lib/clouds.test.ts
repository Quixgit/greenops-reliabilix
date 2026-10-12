import { describe, expect, it } from "vitest";
import { daysBehind } from "@/components/dashboard/cloud-costs";
import { cloudId, cloudMeta } from "./clouds";
import { formatNative } from "./formatters";

describe("cloud identity", () => {
  it("knows the clouds under their usual names and gives a stable colour to an unknown vendor", () => {
    expect(cloudId("Google Cloud")).toBe("gcp");
    expect(cloudId("do")).toBe("digitalocean");
    expect(cloudMeta("aws").name).toBe("AWS");
    expect(cloudMeta("datadog").color).toBe(cloudMeta("datadog").color);
    expect(cloudMeta("datadog").name).toBe("datadog");
  });
});

describe("daysBehind", () => {
  const now = new Date("2026-10-12T09:00:00Z");
  it("is zero up to yesterday (the newest day a bill can be complete for)", () => {
    expect(daysBehind("2026-10-11T00:00:00Z", now)).toBe(0);
    expect(daysBehind("2026-10-10T00:00:00Z", now)).toBe(1);
  });
  it("counts the days a cloud has been silent", () => {
    expect(daysBehind("2026-09-30T00:00:00Z", now)).toBe(11);
    expect(daysBehind(null, now)).toBe(0);
  });
});

describe("formatNative", () => {
  it("names the currency by its code so a dollar is never read as the reporting dollar", () => {
    expect(formatNative(1283, "USD")).toBe("USD 1,283");
    expect(formatNative(1560, "CAD")).toBe("CAD 1,560");
  });
});
