import { describe, expect, it } from "vitest";
import { connectionState, worstState } from "./connection-state";

describe("connectionState", () => {
  it("does not call a connection that never synced connected", () => {
    expect(connectionState({ sync_status: "healthy", last_sync_at: null })).toBe("waiting");
  });
  it("shows a failing sync even though access was verified", () => {
    expect(connectionState({ sync_status: "healthy", last_sync_at: null, last_error: "throttled" })).toBe("failing");
    expect(connectionState({ sync_status: "healthy", last_sync_at: "2026-10-10T10:00:00Z", last_error: "throttled" })).toBe("failing");
  });
  it("needs attention when the customer must act", () => {
    expect(connectionState({ sync_status: "error", last_sync_at: null, last_error: "denied" })).toBe("attention");
  });
  it("is synced after a successful sync", () => {
    expect(connectionState({ sync_status: "healthy", last_sync_at: "2026-10-10T10:00:00Z", last_error: null })).toBe("synced");
  });
  it("takes the worst state of a group", () => {
    expect(worstState(["synced", "waiting"])).toBe("waiting");
    expect(worstState(["synced", "failing", "waiting"])).toBe("failing");
    expect(worstState([])).toBeNull();
  });
});
