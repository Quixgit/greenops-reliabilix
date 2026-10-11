// What the Connectors page and the Overview tile say about every connector, derived from the facts the API gives.
// One place, so the two screens never disagree and a provider's real state (cloud connections, Kubernetes clusters,
// notification destinations) decides what is shown, not a hard-coded list.
import { connectionState, worstState, type ConnectionState } from "./connection-state";

export interface ConnectionFacts {
  id: string; provider: string; account_ref: string; project_id: string; sync_status: string;
  last_sync_at: string | null; last_error?: string | null; access?: string;
}
export interface ClusterFacts { cluster: string; last_hour: string }
export interface DestinationFacts { kind: string; enabled: boolean }
export interface RunFacts { started_at: string; finished_at: string | null; status: string }

/** The state of a connector as a whole: a cloud/cluster/channel state, or "available" when there is none yet. */
export type ConnectorState = ConnectionState | "available";

export interface ConnectorFacts {
  connections: ConnectionFacts[];
  clusters: ClusterFacts[];
  destinations: DestinationFacts[];
  now?: number;
}

/** A cluster that pushed energy data in the last two days counts as connected; an older one is waiting. */
const CLUSTER_FRESH_MS = 48 * 3600_000;

export function clusterState(c: ClusterFacts, now = Date.now()): ConnectionState {
  const t = Date.parse(c.last_hour);
  return Number.isFinite(t) && now - t <= CLUSTER_FRESH_MS ? "synced" : "waiting";
}

export interface ConnectorSummary { state: ConnectorState; count: number; lastAt: string | null }

/** State of one catalog connector id ("aws", "gcp", "azure", "kubernetes", "email", "slack", "webhook"). */
export function connectorSummary(id: string, f: ConnectorFacts): ConnectorSummary {
  const now = f.now ?? Date.now();
  if (id === "aws" || id === "gcp" || id === "azure") {
    const list = f.connections.filter((c) => c.provider === id);
    const last = list.map((c) => c.last_sync_at).filter((x): x is string => !!x).sort().at(-1) ?? null;
    return { state: worstState(list.map(connectionState)) ?? "available", count: list.length, lastAt: last };
  }
  if (id === "kubernetes") {
    const last = f.clusters.map((c) => c.last_hour).sort().at(-1) ?? null;
    return { state: worstState(f.clusters.map((c) => clusterState(c, now))) ?? "available", count: f.clusters.length, lastAt: last };
  }
  if (id === "email" || id === "slack" || id === "webhook") {
    const list = f.destinations.filter((d) => d.kind === id);
    const on = list.filter((d) => d.enabled).length;
    return { state: on > 0 ? "synced" : list.length > 0 ? "waiting" : "available", count: list.length, lastAt: null };
  }
  return { state: "available", count: 0, lastAt: null };
}

export interface Kpis {
  /** Connections and clusters that deliver data now, out of all that exist. */
  active: number; total: number;
  /** Distinct provider ids that deliver data. */
  sources: string[];
  /** Share (0-100) of the connections whose latest finished sync succeeded; null when none has finished one. */
  health: number | null;
  /** Mean duration of the succeeded runs, seconds; null when none. */
  avgSyncSeconds: number | null;
  needsAttention: number;
}

/**
 * `runsById` holds each connection's runs, newest first. Health is read from the latest finished run of each
 * connection: that is what the data is like now (a failure fixed yesterday must not weigh on today), while the
 * history stays visible as the run bars. Durations come from every succeeded run.
 */
export function kpis(f: ConnectorFacts, runsById: Record<string, RunFacts[]>): Kpis {
  const now = f.now ?? Date.now();
  const cloud = f.connections.map((c) => ({ provider: c.provider, state: connectionState(c) }));
  const k8s = f.clusters.map((c) => ({ provider: "kubernetes", state: clusterState(c, now) }));
  const all = [...cloud, ...k8s];
  const finished = (r: RunFacts) => r.status === "succeeded" || r.status === "failed";
  const latest = Object.values(runsById).map((rs) => rs.find(finished)).filter((r): r is RunFacts => !!r);
  const ok = Object.values(runsById).flat().filter((r) => r.status === "succeeded");
  const secs = ok.filter((r) => r.finished_at).map((r) => (Date.parse(r.finished_at!) - Date.parse(r.started_at)) / 1000).filter((s) => s >= 0);
  return {
    active: all.filter((c) => c.state === "synced").length,
    total: all.length,
    sources: [...new Set(all.filter((c) => c.state === "synced").map((c) => c.provider))],
    health: latest.length ? Math.round((latest.filter((r) => r.status === "succeeded").length / latest.length) * 100) : null,
    avgSyncSeconds: secs.length ? secs.reduce((a, b) => a + b, 0) / secs.length : null,
    needsAttention: all.filter((c) => c.state === "attention" || c.state === "failing").length,
  };
}

/** "4.2 min", "35 s", "1.1 h": the unit that keeps the number short. */
export function formatDuration(seconds: number): string {
  if (seconds < 60) return `${Math.max(1, Math.round(seconds))} s`;
  if (seconds < 3600) return `${(seconds / 60).toFixed(seconds < 600 ? 1 : 0)} min`;
  return `${(seconds / 3600).toFixed(1)} h`;
}
