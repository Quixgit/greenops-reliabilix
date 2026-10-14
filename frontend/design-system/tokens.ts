// Single source of truth for chart colours; mirrors the CSS variables in app/globals.css.
// Theme: Flat + Glass + subtle depth (shared with lecode-engine): soft radial background,
// glass cards, blue -> violet primary gradient. Green is the second accent, reserved for carbon.
export const tokens = {
  color: {
    primary: "#3d8b62",
    violet: "#5ea17c",
    carbon: "#14503a",
    carbonSoft: "#cfe8da",
    cost: "#3d8b62",
    warn: "#f59e0b",
    bad: "#e5484d",
    text: "#1c2b24",
    muted: "#7f8f85",
    grid: "#e4ebe1",
  },
  provider: { aws: "#f8a24b", azure: "#4a93e8", gcp: "#62c17b", other: "#b3a8e6" } as Record<string, string>,
} as const;
