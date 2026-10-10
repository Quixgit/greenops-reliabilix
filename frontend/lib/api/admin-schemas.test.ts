import { describe, expect, it } from "vitest";
import * as s from "./schemas";

// The admin screens parse every answer of the Go API. These samples mirror backend/api/openapi.yaml, so a contract
// change that would break a screen fails here first.
describe("admin API contracts", () => {
  it("parses the notification catalog and destinations", () => {
    const cat = s.notificationCatalog.parse({
      channels: ["email", "slack", "webhook"], configured: true, stubbed: ["email"],
      event_types: [{ type: "report.ready", description: "x", default_severity: "info" }],
    });
    expect(cat.stubbed).toEqual(["email"]);
    const d = s.destinations.parse({ items: [{
      id: "1", kind: "webhook", name: "ops", target_hint: "hooks.example.com", event_types: ["*"], min_severity: "info",
      enabled: true, created_by: "u", created_at: "2026-10-09T12:00:00Z",
    }] });
    expect(d[0].target_hint).toBe("hooks.example.com");
    expect(s.createdDestination.parse({ destination: { ...d[0] }, signing_secret: "abc" }).signing_secret).toBe("abc");
  });

  it("rejects an unknown severity instead of rendering garbage", () => {
    expect(() => s.destinations.parse({ items: [{ id: "1", kind: "x", name: "n", target_hint: "h", event_types: [], min_severity: "loud", enabled: true, created_by: "u", created_at: "t" }] })).toThrow();
  });

  it("parses deliveries with nullable fields", () => {
    const [d] = s.deliveries.parse({ items: [{
      id: "1", destination_id: null, kind: "email", destination_name: "me", event_type: "report.ready", severity: "info", status: "skipped",
      attempts: 0, next_attempt_at: "t", last_error: null, created_at: "t", sent_at: null,
    }] });
    expect(d.destination_id).toBeNull();
  });

  it("parses one-time secrets", () => {
    expect(s.apiKeyCreated.parse({ api_key: { id: "1", name: "ci", prefix: "grk_ab", role: "ci", created_by: "u", created_at: "t", last_used_at: null, revoked_at: null }, key: "grk_secret" }).key).toBe("grk_secret");
    expect(s.invitationCreated.parse({ invitation: { id: "1", email: "a@b.c", role: "viewer", created_by: "u", created_at: "t", expires_at: "t", accepted_at: null }, token: "tok" }).token).toBe("tok");
  });

  it("parses the operator console answers", () => {
    expect(s.operatorMe.parse({ subject: "s", email: "e", role: "admin" }).role).toBe("admin");
    const sys = s.operatorSystem.parse({ version: "v", env: "dev", expected_migration: 15, integrations: [{ name: "E-mail (SMTP)", status: "stub", note: "logged" }] });
    expect(sys.integrations[0].status).toBe("stub");
    expect(() => s.operatorSystem.parse({ version: "v", env: "dev", expected_migration: 1, integrations: [{ name: "x", status: "fake", note: "" }] })).toThrow();
    const [e] = s.operatorErrors.parse({ items: [{
      id: "1", service: "worker", component: "job", severity: "error", message: "boom", status: "open", occurrences: 3,
      first_seen: "t", last_seen: "t", tenant_id: null, tenant_name: null, last_request_id: null, handled_by: null, handled_at: null,
    }] });
    expect(e.occurrences).toBe(3);
  });
});

describe("settings engine contracts", () => {
  it("parses a generated settings form", () => {
    const v = s.settingsView.parse({
      groups: [{ id: "security", title: "Security", description: "d" }],
      items: [
        { key: "security.invitation_expiry_days", group: "security", title: "Invitation lifetime", description: "d", scope: "tenant", type: "int", default: 7, value: 14,
          source: "tenant", locked: false, editable: true, unit: "days", min: 1, max: 30, updated_by: "u", updated_at: "2026-10-09T12:00:00Z" },
        { key: "reports.allowed_formats", group: "reports", title: "Formats", description: "d", scope: "tenant", type: "list", default: ["csv"], value: ["csv"],
          source: "default", locked: true, editable: false, list_enum: ["csv", "json"], min_items: 1, max_items: 3, updated_by: null, updated_at: null },
      ],
    });
    expect(v.items[0].value).toBe(14);
    expect(v.items[1].list_enum).toEqual(["csv", "json"]);
  });
  it("rejects an unknown setting type or source", () => {
    const base = { key: "a.b", group: "g", title: "t", description: "d", scope: "tenant", default: 1, value: 1, locked: false, editable: true };
    expect(() => s.settingItem.parse({ ...base, type: "matrix", source: "default" })).toThrow();
    expect(() => s.settingItem.parse({ ...base, type: "int", source: "cloud" })).toThrow();
  });
  it("parses history entries with absent old values", () => {
    const [h] = s.settingChanges.parse({ items: [{ at: "t", tenant_id: null, key: "a.b", old_value: null, new_value: 5, actor: "op", via: "operator" }] });
    expect(h.old_value).toBeNull();
  });
});
