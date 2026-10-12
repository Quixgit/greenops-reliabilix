import { describe, expect, it } from "vitest";
import { clusterState, connectorSummary, formatDuration, gateSource, kpis, type ConnectionFacts } from "./connectors";

const NOW = Date.parse("2026-10-11T12:00:00Z");
const conn = (o: Partial<ConnectionFacts>): ConnectionFacts => ({
  id: "c", provider: "azure", account_ref: "x", project_id: "p", sync_status: "healthy", last_sync_at: "2026-10-11T11:00:00Z", ...o,
});
const facts = (o: Partial<Parameters<typeof connectorSummary>[1]>) => ({ connections: [], clusters: [], destinations: [], now: NOW, ...o });

describe("connectorSummary", () => {
  it("is available until something is connected", () => {
    for (const id of ["aws", "gcp", "azure", "kubernetes", "email", "slack", "webhook"]) expect(connectorSummary(id, facts({})).state).toBe("available");
  });
  it("takes the worst state of a provider's connections", () => {
    const s = connectorSummary("azure", facts({ connections: [conn({ id: "1" }), conn({ id: "2", last_error: "boom" })] }));
    expect(s).toMatchObject({ state: "failing", count: 2, lastAt: "2026-10-11T11:00:00Z" });
    expect(connectorSummary("aws", facts({ connections: [conn({})] })).state).toBe("available"); // another provider
  });
  it("a connection that never synced is waiting, not connected", () => {
    expect(connectorSummary("azure", facts({ connections: [conn({ last_sync_at: null })] })).state).toBe("waiting");
  });
  it("kubernetes follows the clusters that push data", () => {
    expect(connectorSummary("kubernetes", facts({ clusters: [{ cluster: "prod", last_hour: "2026-10-11T10:00:00Z" }] }))).toMatchObject({ state: "synced", count: 1 });
    expect(clusterState({ cluster: "old", last_hour: "2026-10-01T00:00:00Z" }, NOW)).toBe("waiting");
  });
  it("nothing is on while the organization-wide switch is off", () => {
    const f = facts({ destinations: [{ kind: "slack", enabled: true }], notificationsEnabled: false });
    expect(connectorSummary("slack", f).state).toBe("waiting");
  });
  it("a channel counts only while a destination is enabled", () => {
    expect(connectorSummary("slack", facts({ destinations: [{ kind: "slack", enabled: true }] })).state).toBe("synced");
    expect(connectorSummary("slack", facts({ destinations: [{ kind: "slack", enabled: false }] })).state).toBe("waiting");
    expect(connectorSummary("email", facts({ destinations: [{ kind: "slack", enabled: true }] })).state).toBe("available");
  });
});

describe("grid data source", () => {
  it("is live while readings are fresh, waiting when stale, and unknown without any", () => {
    expect(connectorSummary("electricity-maps", facts({ gridReadings: ["2026-10-11T11:00:00Z", "2026-10-11T10:00:00Z"] }))).toMatchObject({ state: "synced", count: 2, lastAt: "2026-10-11T11:00:00.000Z" });
    expect(connectorSummary("electricity-maps", facts({ gridReadings: ["2026-10-09T11:00:00Z"] })).state).toBe("waiting");
    expect(connectorSummary("electricity-maps", facts({})).state).toBe("available");
  });
});

describe("kpis", () => {
  const run = (status: string, start: string, end: string | null) => ({ started_at: start, finished_at: end, status });
  // newest first, as the API lists them: an old failure behind a fresh success must not weigh on health
  const runsById = {
    "1": [run("succeeded", "2026-10-11T10:00:00Z", "2026-10-11T10:02:00Z"), run("failed", "2026-10-10T09:00:00Z", "2026-10-10T09:00:30Z")],
    "2": [run("running", "2026-10-11T11:00:00Z", null), run("failed", "2026-10-11T08:00:00Z", "2026-10-11T08:00:10Z"), run("succeeded", "2026-10-10T07:00:00Z", "2026-10-10T07:04:00Z")],
  };
  it("counts active, sources, health from the latest finished run and the mean duration of succeeded runs", () => {
    const k = kpis(facts({ connections: [conn({ id: "1" }), conn({ id: "2", provider: "aws", sync_status: "error" })], clusters: [{ cluster: "p", last_hour: "2026-10-11T11:00:00Z" }] }), runsById);
    expect(k).toMatchObject({ active: 2, total: 3, health: 50, needsAttention: 1 });
    expect(k.sources.sort()).toEqual(["azure", "kubernetes"]);
    expect(k.avgSyncSeconds).toBe(180);
  });
  it("says nothing it cannot know", () => {
    expect(kpis(facts({}), {})).toMatchObject({ active: 0, total: 0, health: null, avgSyncSeconds: null });
    expect(kpis(facts({ connections: [conn({})] }), { c: [run("running", "2026-10-11T11:00:00Z", null)] }).health).toBeNull();
  });
});

describe("formatDuration", () => {
  it("keeps the number short", () => {
    expect(formatDuration(12)).toBe("12 s");
    expect(formatDuration(252)).toBe("4.2 min");
    expect(formatDuration(1200)).toBe("20 min");
    expect(formatDuration(4000)).toBe("1.1 h");
  });
});

describe("CI connectors", () => {
  const base = { connections: [], clusters: [], destinations: [] };
  it("tells GitHub's and GitLab's runs apart by the label the ready-made snippets send", () => {
    expect(gateSource("acme/web#1234567")).toBe("github-actions");
    expect(gateSource("https://gitlab.com/acme/web/-/pipelines/987")).toBe("gitlab-ci");
    expect(gateSource("console test")).toBeNull();
    expect(gateSource(undefined)).toBeNull();
  });
  it("is connected only by real pipeline runs, and counts them per system", () => {
    const f = { ...base, gate: [{ ref: "acme/web#1", created_at: "2026-10-01T10:00:00Z" }, { ref: "acme/web#2", created_at: "2026-10-02T10:00:00Z" }, { ref: "x", created_at: "2026-10-03T10:00:00Z" }] };
    expect(connectorSummary("github-actions", f)).toEqual({ state: "synced", count: 2, lastAt: "2026-10-02T10:00:00Z" });
    expect(connectorSummary("gitlab-ci", f)).toEqual({ state: "available", count: 0, lastAt: null });
    expect(connectorSummary("github-actions", base).state).toBe("available");
  });
  it("shows Prometheus as the Kubernetes clusters that read it", () => {
    const f = { ...base, clusters: [{ cluster: "prod", last_hour: new Date().toISOString() }] };
    expect(connectorSummary("prometheus", f).state).toBe("synced");
    expect(connectorSummary("prometheus", base).state).toBe("available");
  });
});
