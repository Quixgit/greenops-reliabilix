"use client";

import { useId } from "react";
import { cn } from "@/lib/utils";

/** A small filled line that draws itself in: the shape of the last days behind a KPI. Needs two points to say anything. */
export function MiniArea({ values, className, stroke = "#9fb4d3", height = 44 }: { values: number[]; className?: string; stroke?: string; height?: number }) {
  const id = useId();
  if (values.length < 2) return null;
  const W = 200, H = height, min = Math.min(...values), max = Math.max(...values), span = max - min || 1;
  const pts = values.map((v, i) => [(i / (values.length - 1)) * W, H - 4 - ((v - min) / span) * (H - 10)] as const);
  const line = pts.map(([x, y], i) => `${i ? "L" : "M"}${x.toFixed(1)},${y.toFixed(1)}`).join(" ");
  return (
    <svg viewBox={`0 0 ${W} ${H}`} preserveAspectRatio="none" className={cn("block h-11 w-full", className)} aria-hidden>
      <defs>
        <linearGradient id={id} x1="0" y1="0" x2="0" y2="1"><stop offset="0" stopColor={stroke} stopOpacity=".28" /><stop offset="1" stopColor={stroke} stopOpacity="0" /></linearGradient>
      </defs>
      <path d={`${line} L${W},${H} L0,${H} Z`} fill={`url(#${id})`} className="fade-in" />
      <path d={line} fill="none" stroke={stroke} strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" vectorEffect="non-scaling-stroke"
        pathLength={100} className="draw" style={{ ["--len" as string]: 100 }} />
    </svg>
  );
}
