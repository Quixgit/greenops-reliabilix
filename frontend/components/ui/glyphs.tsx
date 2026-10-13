import { cn } from "@/lib/utils";

/**
 * The Overview's glyphs in the DX family: the same 24-unit grid and pen as the icon set (components/ui/icons.tsx), no fill, no second tone.
 * They take the text colour; the flat tint behind them comes from the tile (see IconTile).
 */
const G: Record<string, string> = {
  wallet: "M3 6h18v14H3z M3 10.5h18 M15 15h3",
  leaf: "M4 20v-8l8-8h8v8l-8 8z M4 20L14 10",
  bolt: "M13.5 2L5 14h6l-1.5 8L19 10h-6.5z",
  gauge: "M3 18l2.5-7.5L12 6l6.5 4.5L21 18z M12 18l4-6",
  calendar: "M3 5h18v16H3z M3 10h18 M8 3v4 M16 3v4 M7 17l3-3 3 2 4-5",
  pie: "M3 3h18v18H3z M12 3v9h9",
  pulse: "M2 12h5l3-8 4 16 3-8h5",
  scale: "M12 3v18 M5 21h14 M4 7h16 M4 7l-2.5 7h5z M20 7l-2.5 7h5z",
  layers: "M12 3l9 5-9 5-9-5z M3 12.5l9 5 9-5 M3 17l9 5 9-5",
  bars: "M4 20h16 M6 20v-7 M11 20V8 M16 20V10",
  trend: "M3 3v18h18 M6 16l4-5 3 3 6-8",
  cloud: "M6 18h12l3.5-3.5-2-4.5h-4L12.5 6 8 9.5l-4 1.5z",
  pin: "M12 21.5l-6-8V3.5h12v10z M10 7.5h4v4h-4z",
  shield: "M12 2.5l8 3v7c0 5-3.5 8-8 9-4.5-1-8-4-8-9v-7z M8.5 12l2.5 3 4.5-5",
  spark: "M12 3l2 6 6 2-6 2-2 6-2-6-6-2 6-2z M19 3v4 M17 5h4",
  list: "M8 6h13 M8 12h13 M8 18h13 M3 5h3v3H3z M3 11h3v3H3z M3 17h3v3H3z",
  unit: "M4 4h16v16H4z M8.5 12.5l2.5 2.5 4.5-5",
  bell: "M12 3l6.5 4v6l2 3.5h-17l2-3.5V7z M9.5 20h5",
};

export type GlyphName = keyof typeof G;

export function Glyph({ name, className }: { name: GlyphName; className?: string }) {
  return (
    <svg viewBox="0 0 24 24" className={cn("size-5 shrink-0", className)} fill="none" stroke="currentColor" strokeWidth="1.6" strokeLinecap="square" strokeLinejoin="miter" aria-hidden>
      <path d={G[name]} />
    </svg>
  );
}

export type Tone = "blue" | "green" | "amber" | "violet" | "rose" | "sky" | "slate";

/** Kept for the pages that still ask for a tile: it draws only the glyph, in ink, with no box behind it. */
export function IconTile({ name, size = "md", className }: { name: GlyphName; tone?: Tone; size?: "sm" | "md" | "lg"; className?: string }) {
  return <Glyph name={name} className={cn("dx-ic dx-ic-lg text-[var(--dx-ink-2)]", size === "sm" ? "size-[18px]" : "size-5", className)} />;
}
