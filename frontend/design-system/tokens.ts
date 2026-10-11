// Single source of truth for chart colours; mirrors the CSS variables in app/globals.css.
// Theme: Flat + Glass + subtle depth (shared with lecode-engine): soft radial background,
// glass cards, blue -> violet primary gradient. Green is the second accent, reserved for carbon.
export const tokens = {
  color: {
    primary: "#3b7cf4",
    violet: "#7c5cf0",
    carbon: "#6fc3a8",
    carbonSoft: "#bff0dc",
    cost: "#3b7cf4",
    warn: "#f59e0b",
    bad: "#e5484d",
    text: "#0f172a",
    muted: "#6b7a94",
    grid: "#e6ebf4",
  },
  provider: { aws: "#f8a24b", azure: "#4a93e8", gcp: "#62c17b", other: "#b3a8e6" } as Record<string, string>,
} as const;
