import { tokens } from "@/design-system/tokens";

/** Tiny dependency-free SVG sparkline that draws itself in. Nothing meaningful for <2 points (honest: no trend yet). */
export function Sparkline({ values, width = 84, height = 26, delay = 0 }: { values: number[]; width?: number; height?: number; delay?: number }) {
  if (values.length < 2) return <span className="text-xs text-muted">—</span>;
  const min = Math.min(...values), max = Math.max(...values), span = max - min || 1;
  const pts = values.map((v, i) => [(i / (values.length - 1)) * width, height - 3 - ((v - min) / span) * (height - 6)] as const);
  // Smooth the polyline with quadratic midpoints so the line flows instead of zig-zagging.
  let d = `M${pts[0][0]},${pts[0][1]}`;
  for (let i = 1; i < pts.length; i++) {
    const [x0, y0] = pts[i - 1], [x1, y1] = pts[i];
    d += ` Q${x0},${y0} ${(x0 + x1) / 2},${(y0 + y1) / 2}`;
  }
  d += ` L${pts.at(-1)![0]},${pts.at(-1)![1]}`;
  return (
    <svg width={width} height={height} viewBox={`0 0 ${width} ${height}`} role="img" aria-label="CO₂e trend" className="overflow-visible">
      <path d={d} fill="none" stroke={tokens.color.primary} strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" pathLength={200}
        className="draw" style={{ ["--len" as string]: 200, ["--d" as string]: `${delay}ms` }} />
    </svg>
  );
}
