import type { ReactNode } from "react";
import { cn } from "@/lib/utils";

/**
 * The Overview's own glyphs: a soft filled shape under a single round stroke, drawn on one 24-unit grid, so a tile reads at 20 px and still looks
 * drawn rather than stock. They take the text colour; the tint behind them comes from the tile (see IconTile).
 */
const G: Record<string, { fill?: ReactNode; line: ReactNode }> = {
  wallet: {
    fill: <path d="M4.5 7.5a2.5 2.5 0 0 1 2.500-2.500h10.500a2 2 0 0 1 2 2V19a1.500 1.500 0 0 1-1.500 1.500H7a2.500 2.500 0 0 1-2.500-2.500z" />,
    line: <><path d="M4.500 8v10a2.500 2.500 0 0 0 2.500 2.500h11a1.500 1.500 0 0 0 1.500-1.500v-9a1.500 1.500 0 0 0-1.500-1.500H7a2.500 2.500 0 0 1-2.500-2.500 2.500 2.500 0 0 1 2.500-2.500h9.500" /><circle cx="16.200" cy="14.200" r="1.150" fill="currentColor" stroke="none" /></>,
  },
  leaf: {
    fill: <path d="M5.200 19.200C4.500 11.500 9.500 5.500 19.500 4.800c.4 8.600-4.600 14.300-12.500 14.600z" />,
    line: <><path d="M5.200 19.200C4.500 11.500 9.500 5.500 19.500 4.800c.4 8.600-4.600 14.300-12.500 14.600z" /><path d="M5.500 19c2.800-4.600 6-7.800 10-10.200" /></>,
  },
  bolt: {
    fill: <path d="M13.500 3 5 13.500h6L10 21l9-11h-6.200z" />,
    line: <path d="M13.500 3 5 13.500h6L10 21l9-11h-6.200z" />,
  },
  gauge: {
    fill: <path d="M3.800 16a8.200 8.200 0 1 1 16.400 0z" />,
    line: <><path d="M3.800 16a8.200 8.200 0 1 1 16.400 0" /><path d="M12 16l3.800-5" /><circle cx="12" cy="16" r="1.250" fill="currentColor" stroke="none" /><path d="M5 20h14" /></>,
  },
  calendar: {
    fill: <rect x="3.500" y="5.500" width="17" height="15" rx="3.200" />,
    line: <><rect x="3.500" y="5.500" width="17" height="15" rx="3.200" /><path d="M3.800 10h16.400M8 3.500v3.500M16 3.500v3.500" /><path d="M7.500 17l3-3 2.400 2 3.600-4" /></>,
  },
  pie: {
    fill: <circle cx="12" cy="12" r="8.600" />,
    line: <><circle cx="12" cy="12" r="8.600" /><path d="M12 3.400V12h8.600" /></>,
  },
  pulse: {
    fill: <path d="M3 12h4l2-6 4 12 2-6h6v8H3z" opacity=".5" />,
    line: <path d="M3 12h4l2-6 4 12 2-6h6" />,
  },
  scale: {
    fill: <path d="M5 8.500 2.800 14a3.200 3.200 0 0 0 6.400 0zM19 8.500 16.800 14a3.200 3.200 0 0 0 6.400 0z" />,
    line: <><path d="M12 4v16M6.500 20h11M5 8.500h14" /><path d="M5 8.500 2.800 14a3.200 3.200 0 0 0 4.400 0M19 8.500 16.800 14a3.200 3.200 0 0 0 4.400 0" /></>,
  },
  layers: {
    fill: <path d="M12 3.500 3.500 8.200 12 13l8.500-4.800z" />,
    line: <><path d="M12 3.500 3.500 8.200 12 13l8.500-4.800z" /><path d="M3.500 12.200 12 17l8.500-4.800M3.500 16 12 20.800 20.500 16" /></>,
  },
  bars: {
    fill: <><rect x="4" y="12" width="4" height="8" rx="1.200" /><rect x="10" y="7" width="4" height="13" rx="1.200" /><rect x="16" y="4" width="4" height="16" rx="1.200" /></>,
    line: <><rect x="4" y="12" width="4" height="8" rx="1.200" /><rect x="10" y="7" width="4" height="13" rx="1.200" /><rect x="16" y="4" width="4" height="16" rx="1.200" /></>,
  },
  trend: {
    fill: <path d="M3 18 9 11l4 4 8-9v14H3z" opacity=".55" />,
    line: <><path d="M3 18 9 11l4 4 8-9" /><path d="M15.500 6H21v5.500" /></>,
  },
  cloud: {
    fill: <path d="M7.500 18.500a4.500 4.500 0 0 1-.6-8.960A6 6 0 0 1 18.400 11 3.800 3.800 0 0 1 17.500 18.500z" />,
    line: <path d="M7.500 18.500a4.500 4.500 0 0 1-.6-8.960A6 6 0 0 1 18.400 11a3.800 3.800 0 0 1-.9 7.500z" />,
  },
  pin: {
    fill: <path d="M12 21s7-6 7-11.500a7 7 0 0 0-14 0C5 15 12 21 12 21z" />,
    line: <><path d="M12 21s7-6 7-11.500a7 7 0 0 0-14 0C5 15 12 21 12 21z" /><circle cx="12" cy="9.500" r="2.400" /></>,
  },
  shield: {
    fill: <path d="M12 3 4.500 6v6c0 4.400 3.200 7.700 7.500 9 4.300-1.300 7.500-4.600 7.500-9V6z" />,
    line: <><path d="M12 3 4.500 6v6c0 4.400 3.200 7.700 7.500 9 4.300-1.300 7.500-4.600 7.500-9V6z" /><path d="m8.800 12 2.300 2.300 4.200-4.500" /></>,
  },
  spark: {
    fill: <path d="M11 3.500 13 9l5.500 2-5.500 2-2 5.500L9 13l-5.500-2L9 9z" />,
    line: <><path d="M11 3.500 13 9l5.500 2-5.500 2-2 5.500L9 13l-5.500-2L9 9z" /><path d="M19 3.500v3M17.500 5h3" /></>,
  },
  list: {
    line: <><path d="M9 6.500h11M9 12h11M9 17.500h11" /><circle cx="4.600" cy="6.500" r="1.100" fill="currentColor" stroke="none" /><circle cx="4.600" cy="12" r="1.100" fill="currentColor" stroke="none" /><circle cx="4.600" cy="17.500" r="1.100" fill="currentColor" stroke="none" /></>,
  },
  unit: {
    fill: <rect x="4" y="4" width="16" height="16" rx="4" />,
    line: <><rect x="4" y="4" width="16" height="16" rx="4" /><path d="M8.500 12.200 11 14.600l4.600-5" /></>,
  },
  bell: {
    fill: <path d="M6 16.500V11a6 6 0 0 1 12 0v5.500l1.500 2h-15z" />,
    line: <><path d="M6 16.500V11a6 6 0 0 1 12 0v5.500l1.500 2h-15z" /><path d="M10 21a2.200 2.200 0 0 0 4 0" /></>,
  },
};

export type GlyphName = keyof typeof G;

export function Glyph({ name, className }: { name: GlyphName; className?: string }) {
  const g = G[name];
  return (
    <svg viewBox="0 0 24 24" className={cn("size-5 shrink-0", className)} fill="none" stroke="currentColor" strokeWidth="1.6" strokeLinecap="round" strokeLinejoin="round" aria-hidden>
      {g.fill && <g fill="currentColor" stroke="none" opacity=".16">{g.fill}</g>}
      {g.line}
    </svg>
  );
}

export type Tone = "blue" | "green" | "amber" | "violet" | "rose" | "sky" | "slate";
const TONE: Record<Tone, string> = {
  blue: "from-[#e7f0fd] to-[#f6f9ff] text-[#4a7fd0] ring-[#d3e3fa]",
  green: "from-[#e4f6ef] to-[#f5fbf8] text-[#2f8f78] ring-[#cdeadf]",
  amber: "from-[#fdf1d9] to-[#fffaf0] text-[#c0872a] ring-[#f6e2b6]",
  violet: "from-[#eeebfb] to-[#f8f7fe] text-[#7461d4] ring-[#ddd7f6]",
  rose: "from-[#fcebee] to-[#fff7f8] text-[#c4647a] ring-[#f6d3da]",
  sky: "from-[#e1f4fb] to-[#f4fbfe] text-[#3b93b8] ring-[#c9e8f4]",
  slate: "from-[#eef1f6] to-[#f8fafc] text-[#64748b] ring-[#dfe5ee]",
};

/** A glyph on a soft gradient tile: the one way an icon appears on the Overview. */
export function IconTile({ name, tone = "blue", size = "md", className }: { name: GlyphName; tone?: Tone; size?: "sm" | "md" | "lg"; className?: string }) {
  return (
    <span className={cn("grid shrink-0 place-items-center rounded-xl bg-gradient-to-br ring-1 transition-transform duration-500 group-hover/tile:scale-105 group-hover/tile:-rotate-3",
      size === "sm" ? "size-8" : size === "lg" ? "size-11" : "size-10", TONE[tone], className)}>
      <Glyph name={name} className={size === "sm" ? "size-[18px]" : "size-[22px]"} />
    </span>
  );
}
