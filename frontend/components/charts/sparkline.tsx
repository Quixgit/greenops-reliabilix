import { tokens } from "@/design-system/tokens";

/** Tiny dependency-free SVG sparkline. Renders nothing meaningful for <2 points (honest: no trend yet). */
export function Sparkline({ values, width = 84, height = 26 }: { values: number[]; width?: number; height?: number }) {
  if (values.length < 2) return <span className="text-xs text-muted">—</span>;
  const min = Math.min(...values), max = Math.max(...values), span = max - min || 1;
  const pts = values.map((v, i) => `${(i / (values.length - 1)) * width},${height - 2 - ((v - min) / span) * (height - 4)}`).join(" ");
  return (
    <svg width={width} height={height} viewBox={`0 0 ${width} ${height}`} role="img" aria-label="CO₂e trend">
      <polyline points={pts} fill="none" stroke={tokens.color.carbon} strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" />
    </svg>
  );
}
