"use client";

import type { CSSProperties, ReactNode } from "react";
import { MiniArea } from "@/components/charts/mini-area";
import { Card } from "@/components/ui/card";
import { IconTile, type GlyphName, type Tone } from "@/components/ui/glyphs";
import { Info } from "@/components/ui/info";
import { Skeleton } from "@/components/ui/skeleton";
import { cn } from "@/lib/utils";

/**
 * One figure of the Overview, compact: a tinted glyph, the label, the number with its change beside it, one line of context under it and, when
 * there is a series, its shape on the right. Every KPI of the page is this tile, so they line up and read the same.
 */
export function Tile({ index, icon, tone, label, info, value, unit, delta, foot, spark, sparkColor, loading, empty, className }: {
  index?: number; icon: GlyphName; tone: Tone; label: string; info?: string; value: ReactNode | null; unit?: string; delta?: ReactNode;
  foot?: ReactNode; spark?: number[]; sparkColor?: string; loading?: boolean; empty?: ReactNode; className?: string;
}) {
  return (
    <Card index={index} className={cn("group/tile @container/tile flex min-w-0 flex-col gap-2 p-3", className)} style={{ "--i": index ?? 0 } as CSSProperties}>
      {info && <Info corner>{info}</Info>}
      <div className="flex items-center gap-3 pr-5">
        <IconTile name={icon} tone={tone} />
        <div className="min-w-0 flex-1">
          <p className="flex items-center gap-1 truncate text-xs font-medium text-slate-500">{label}</p>
          {loading ? <Skeleton className="mt-1 h-7 w-24" /> : value === null ? <p className="text-xl font-bold leading-7 text-slate-300">—</p> : (
            <div className="flex flex-wrap items-baseline gap-x-1.5 text-[22px] font-bold leading-7 tracking-tight text-ink tabular-nums">
              <span className="whitespace-nowrap">{value}{unit && <span className="ml-1 text-xs font-medium text-slate-500">{unit}</span>}</span>
              {delta}
            </div>
          )}
        </div>
        {!loading && value !== null && spark && spark.length > 1 && <div className="hidden h-9 w-20 shrink-0 @[18.5rem]/tile:block"><MiniArea values={spark} stroke={sparkColor} height={36} className="h-9" /></div>}
      </div>
      <div className="min-h-[2rem] text-[11.5px] leading-snug text-slate-600">
        {loading ? <Skeleton className="h-3.5 w-40" /> : value === null ? (empty ?? "") : foot}
      </div>
    </Card>
  );
}
