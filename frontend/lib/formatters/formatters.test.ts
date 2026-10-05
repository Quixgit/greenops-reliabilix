import { describe, expect, it } from "vitest";
import { formatCarbon, formatCost, pctChange, timeAgo } from "./index";

describe("formatters", () => {
  it("formats carbon in kg and tonnes", () => {
    expect(formatCarbon(320)).toBe("320 kg");
    expect(formatCarbon(42600)).toBe("42.6 t");
  });
  it("formats cost", () => expect(formatCost(8432)).toBe("$8,432"));
  it("pctChange is null without a baseline", () => {
    expect(pctChange(10, 0)).toBeNull();
    expect(pctChange(90, 100)).toBe(-10);
  });
  it("timeAgo", () => {
    const now = Date.parse("2026-10-05T12:00:00Z");
    expect(timeAgo("2026-10-05T10:00:00Z", now)).toBe("2h ago");
    expect(timeAgo("2026-10-05T11:59:50Z", now)).toBe("just now");
  });
});
