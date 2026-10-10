/**
 * What a cloud connection is really doing, from the three facts the API gives. "healthy" only means the access check
 * passed: a connection that never synced, or whose last attempt failed, is not "connected" in the sense that matters
 * (no data), and must not look it.
 */
export type ConnectionState = "attention" | "failing" | "waiting" | "synced";

export function connectionState(c: { sync_status: string; last_sync_at: string | null; last_error?: string | null }): ConnectionState {
  if (c.sync_status === "error") return "attention";
  if (c.last_error) return "failing";
  if (!c.last_sync_at) return "waiting";
  return "synced";
}

/** Badge tone of a state (attention and failing are bad, waiting is a warning). */
export function stateTone(s: ConnectionState): "good" | "warn" | "bad" {
  return s === "synced" ? "good" : s === "waiting" ? "warn" : "bad";
}

/** The worst state of a group: attention, then failing, then waiting, then synced. */
export function worstState(states: ConnectionState[]): ConnectionState | null {
  for (const s of ["attention", "failing", "waiting", "synced"] as const) if (states.includes(s)) return s;
  return null;
}
