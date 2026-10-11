"use client";

import { useEffect, useState } from "react";
import { cn } from "@/lib/utils";

/** A small ring that fills to `value` (0-100) when it appears. No value draws an empty ring, never a made-up one. */
export function HealthRing({ value, size = 44, className }: { value: number | null; size?: number; className?: string }) {
  const [shown, setShown] = useState(0);
  useEffect(() => { const id = requestAnimationFrame(() => setShown(value ?? 0)); return () => cancelAnimationFrame(id); }, [value]);
  const r = (size - 6) / 2, c = 2 * Math.PI * r;
  const tone = value === null ? "stroke-slate-300" : value >= 90 ? "stroke-emerald-500" : value >= 60 ? "stroke-amber-500" : "stroke-red-500";
  return (
    <svg width={size} height={size} viewBox={`0 0 ${size} ${size}`} className={cn("-rotate-90", className)} role="img" aria-label={value === null ? "no data" : `${value}%`}>
      <circle cx={size / 2} cy={size / 2} r={r} fill="none" strokeWidth={5} className="stroke-slate-100" />
      <circle
        cx={size / 2} cy={size / 2} r={r} fill="none" strokeWidth={5} strokeLinecap="round"
        className={cn("ring-arc", tone)} strokeDasharray={c} strokeDashoffset={c * (1 - shown / 100)}
      />
    </svg>
  );
}
