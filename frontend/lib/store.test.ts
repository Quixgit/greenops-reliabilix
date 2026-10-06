import { describe, expect, it } from "vitest";
import { periodRange } from "./store";

describe("periodRange", () => {
  it("is inclusive and ends today", () => {
    expect(periodRange("7d", new Date("2026-10-05T15:00:00Z"))).toEqual({ from: "2026-09-29", to: "2026-10-05" });
  });
});
