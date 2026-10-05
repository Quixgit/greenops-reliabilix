import { Lightbulb } from "lucide-react";
import { formatCarbon, formatCost } from "@/lib/formatters";

export type RecommendationView = {
  id: string; title: string; provider: string; impact: "low" | "medium" | "high";
  costDelta: number; carbonDeltaKg: number; currency?: string;
};

/**
 * Ready for phase 2 (GET /api/v1/recommendations). Not rendered yet: until the recommendations
 * domain exists the panel shows an honest empty state instead of invented savings.
 */
export function RecommendationCard({ r }: { r: RecommendationView }) {
  return (
    <article className="flex items-start gap-3 rounded-xl bg-white/60 p-3 ring-1 ring-line">
      <span className="flex size-9 shrink-0 items-center justify-center rounded-lg bg-carbon-soft text-carbon"><Lightbulb className="size-4" /></span>
      <div className="min-w-0 flex-1">
        <h3 className="truncate text-sm font-semibold">{r.title}</h3>
        <p className="mt-0.5 flex gap-1.5 text-xs text-muted"><span className="rounded bg-slate-100 px-1.5 py-0.5">{r.provider}</span><span className="rounded bg-slate-100 px-1.5 py-0.5">{r.impact} impact</span></p>
      </div>
      <dl className="shrink-0 text-right text-xs">
        <dd className="font-semibold text-primary">{formatCost(r.costDelta, r.currency)}/mo</dd>
        <dd className="font-semibold text-carbon">{formatCarbon(r.carbonDeltaKg)} CO₂e</dd>
      </dl>
    </article>
  );
}
