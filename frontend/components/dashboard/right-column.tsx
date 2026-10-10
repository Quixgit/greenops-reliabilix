"use client";

import { ArrowRight, Leaf, Lightbulb, MapPin, Server, Sparkles, type LucideIcon } from "lucide-react";
import Link from "next/link";
import type { CSSProperties } from "react";
import { Card, CardHeader } from "@/components/ui/card";
import { EmptyState, ErrorState } from "@/components/ui/empty-state";
import { Skeleton } from "@/components/ui/skeleton";
import { content } from "@/content/schema";
import { useRecommendations } from "@/hooks/use-overview";
import type { Recommendation } from "@/lib/api/schemas";
import { formatCost } from "@/lib/formatters";
import { cn } from "@/lib/utils";

/** Hero banner: light sky and layered mountains on the right, copy on the left (no text over a busy picture). */
export function PromoBanner() {
  const b = content.promoBanner;
  return (
    <Card index={1} interactive={false} className="group relative overflow-hidden border-0 p-0 shadow-[0_8px_30px_rgba(31,52,110,0.10)]">
      <div className="absolute inset-0 bg-[#cfdcf3] bg-cover bg-center transition-transform duration-[1800ms] ease-[var(--ease-out-soft)] group-hover:scale-105" style={{ backgroundImage: `url(${b.imageUrl})` }} aria-hidden />
      <div className="absolute inset-0 bg-gradient-to-r from-white/75 via-white/25 to-transparent" aria-hidden />
      <div className="relative flex min-h-[204px] flex-col justify-center gap-2.5 p-6">
        <h2 className="text-[22px] font-bold tracking-tight text-ink">{b.title}</h2>
        <p className="max-w-[230px] text-[13px] leading-snug text-slate-600">{b.text}</p>
        <Link href="/recommendations" className="mt-1.5 inline-flex w-fit items-center gap-2 rounded-xl bg-navy-active px-4 py-2.5 text-sm font-semibold text-white shadow-lg shadow-slate-900/20 transition-all duration-300 hover:-translate-y-0.5 hover:bg-navy hover:shadow-xl">
          {b.cta} <ArrowRight className="size-4 transition-transform duration-300 group-hover:translate-x-1" />
        </Link>
      </div>
    </Card>
  );
}

const ICONS: Record<string, { icon: LucideIcon; tone: string }> = {
  region_shift: { icon: MapPin, tone: "bg-emerald-50 text-emerald-600" },
  rightsizing: { icon: Server, tone: "bg-violet-50 text-violet-600" },
};
const FALLBACK = { icon: Lightbulb, tone: "bg-amber-50 text-amber-600" };

/** Impact tier derived from the estimated carbon reduction: the rule is shown in the badge's tooltip. */
function impactOf(r: Recommendation): { label: string; cls: string } {
  const p = r.estimated_carbon_reduction_pct;
  if (p >= 40) return { label: "High impact", cls: "bg-orange-50 text-orange-700 ring-orange-200" };
  if (p >= 15) return { label: "Medium impact", cls: "bg-amber-50 text-amber-700 ring-amber-200" };
  return { label: "Low impact", cls: "bg-slate-100 text-slate-600 ring-slate-200" };
}

export function TopRecommendations() {
  const q = useRecommendations("open");
  const top = [...(q.data ?? [])].sort((a, b) => b.carbon_reduction_kg_month - a.carbon_reduction_kg_month).slice(0, 5);
  return (
    <Card index={2} interactive={false}>
      <CardHeader icon={<Sparkles className="size-[18px] text-blue-500" />} title="Top Recommendations"
        action={<Link href="/recommendations" className="text-xs font-semibold text-primary hover:underline">View all →</Link>} />
      {q.isLoading ? <Skeleton className="h-56" /> : q.isError ? <ErrorState /> : top.length === 0 ? <EmptyState icon={Lightbulb} text={content.recommendationsEmpty} /> : (
        <ul className="divide-y divide-line/70">
          {top.map((r, i) => {
            const m = ICONS[r.type] ?? FALLBACK, impact = impactOf(r);
            return (
              <li key={r.id} className="rise group" style={{ "--i": i + 3 } as CSSProperties}>
                <Link href="/recommendations" className="flex items-center gap-3 rounded-xl py-3 transition-colors hover:bg-slate-50/80">
                  <span className={cn("flex size-11 shrink-0 items-center justify-center rounded-xl transition-transform duration-300 group-hover:scale-110 group-hover:-rotate-3", m.tone)}><m.icon className="size-5" strokeWidth={1.8} /></span>
                  <span className="min-w-0 flex-1">
                    <span className="block truncate text-sm font-medium">{r.title}</span>
                    <span className="mt-1 flex flex-wrap gap-1.5">
                      <span className="rounded-md bg-slate-100 px-1.5 py-0.5 text-[11px] font-medium uppercase text-slate-600">{r.provider}</span>
                      <span title="High: 40% or more estimated carbon reduction, medium: 15% or more" className={cn("rounded-md px-1.5 py-0.5 text-[11px] font-medium ring-1 ring-inset", impact.cls)}>{impact.label}</span>
                    </span>
                  </span>
                  <span className="shrink-0 text-right leading-tight">
                    {r.estimated_cost_impact === null
                      ? <span className="block text-xs text-muted" title="No price data for this change">cost n/a</span>
                      : <span className="block text-sm font-semibold">{r.estimated_cost_impact <= 0 ? "−" : "+"}{formatCost(Math.abs(r.estimated_cost_impact))}/mo</span>}
                    <span className="mt-0.5 flex items-center justify-end gap-1 text-xs font-medium text-emerald-600"><Leaf className="size-3" />−{r.carbon_reduction_kg_month.toFixed(1)} kg CO₂e</span>
                  </span>
                </Link>
              </li>
            );
          })}
        </ul>
      )}
    </Card>
  );
}

export const Tagline = () => (
  <p className="flex items-center justify-end gap-1.5 pt-1 text-xs text-muted">{content.tagline}<Leaf className="size-3.5 text-emerald-500" /></p>
);
