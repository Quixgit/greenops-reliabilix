// Single source of truth for chart colours; mirrors the CSS variables in app/globals.css.
// Theme: Flat + Glass + subtle depth (shared with lecode-engine): soft radial background,
// glass cards, blue -> violet primary gradient. Green is the second accent, reserved for carbon.
export const tokens = {
  color: {
    primary: "#4f6df5",
    violet: "#7c4dff",
    carbon: "#16a34a",
    carbonSoft: "#86efac",
    cost: "#4f6df5",
    warn: "#f59e0b",
    bad: "#e5484d",
    text: "#0f172a",
    muted: "#64748b",
    grid: "#e2e8f0",
  },
  provider: { aws: "#f59e0b", azure: "#2563eb", gcp: "#ef4444", other: "#94a3b8" } as Record<string, string>,
} as const;
