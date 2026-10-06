import { ArrowDown, ArrowUp } from "lucide-react";
import { cn } from "@/lib/utils";
import { formatPct } from "@/lib/formatters";

/**
 * Change vs the previous period. `better` says which direction is good for the metric:
 * CO2e, cost, carbon intensity and SCI (carbon per functional unit) all improve when they go DOWN,
 * so the colour follows "is this a good change", not the arrow direction.
 */
export function DeltaBadge({ pct, better = "lower" }: { pct: number | null; better?: "lower" | "higher" }) {
  if (pct === null) return null;
  if (Math.abs(pct) < 0.05) return <span className="inline-flex rounded-full bg-slate-100 px-2 py-0.5 text-xs font-semibold text-muted">0%</span>;
  const good = better === "lower" ? pct <= 0 : pct >= 0;
  const Arrow = pct <= 0 ? ArrowDown : ArrowUp;
  return (
    <span className={cn("inline-flex items-center gap-0.5 rounded-full px-2 py-0.5 text-xs font-semibold",
      good ? "bg-carbon-soft text-carbon" : "bg-red-50 text-bad")}>
      <Arrow className="size-3" />{formatPct(pct)}
    </span>
  );
}
