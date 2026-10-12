"use client";

import { latencyBasisLabel } from "@/lib/latency-basis";
import { IconServer as Server, IconArrowRight as ArrowRight, IconBulb as Lightbulb, IconLeaf as Leaf, IconPin as MapPin } from "@/components/ui/icons";
import type { AnyIcon as LucideIcon } from "@/components/ui/icons";
import Link from "next/link";
import { useState, type CSSProperties } from "react";
import { Glyph } from "@/components/ui/glyphs";
import { Info } from "@/components/ui/info";
import { cloudMeta } from "@/lib/clouds";
import { Card } from "@/components/ui/card";
import { AllowRegions } from "./allow-regions";
import { EmptyState, ErrorState } from "@/components/ui/empty-state";
import { Skeleton } from "@/components/ui/skeleton";
import { content } from "@/content/schema";
import { regionName } from "@/lib/regions";
import { useAutomationJobs, useFinopsSummary, useOptimization, useRecommendations } from "@/hooks/use-overview";
import { useT } from "@/i18n/provider";
import type { Recommendation } from "@/lib/api/schemas";
import { formatCarbon, formatCost } from "@/lib/formatters";
import { cn } from "@/lib/utils";

/** Hero banner: light sky and layered mountains on the right, copy on the left (no text over a busy picture). */
export function PromoBanner() {
  const b = content.promoBanner;
  return (
    <Card index={1} interactive={false} className="group relative overflow-hidden p-0 shadow-[0_2px_10px_rgba(31,52,110,0.05)]">
      <div className="absolute inset-0 bg-[#cfdcf3] bg-cover bg-center transition-transform duration-[1800ms] ease-[var(--ease-out-soft)] group-hover:scale-105" style={{ backgroundImage: `url(${b.imageUrl})` }} aria-hidden />
      <div className="absolute inset-0 bg-gradient-to-r from-white/75 via-white/25 to-transparent" aria-hidden />
      <div className="relative flex min-h-[204px] flex-col justify-center gap-2.5 p-6">
        <h2 className="text-[22px] font-bold tracking-tight text-ink">{b.title}</h2>
        <p className="max-w-[230px] text-[13px] leading-snug text-slate-600">{b.text}</p>
        <Link href="/recommendations" className="mt-1.5 inline-flex w-fit items-center gap-2 rounded-xl bg-navy-active px-4 py-2.5 text-sm font-semibold text-white shadow-sm shadow-slate-900/10 transition-all duration-300 hover:-translate-y-0.5 hover:bg-navy hover:shadow-md">
          {b.cta} <ArrowRight className="size-4 transition-transform duration-300 group-hover:translate-x-1" />
        </Link>
      </div>
    </Card>
  );
}

const ICONS: Record<string, { icon: LucideIcon; tone: string }> = {
  region_shift: { icon: MapPin, tone: "bg-slate-100 text-slate-500" },
  rightsizing: { icon: Server, tone: "bg-slate-100 text-slate-500" },
};
const FALLBACK = { icon: Lightbulb, tone: "bg-slate-100 text-slate-500" };

/** Impact tier derived from the estimated carbon reduction: the rule is shown in the badge's tooltip. */
function impactOf(r: Recommendation): { label: string; cls: string } {
  const p = r.estimated_carbon_reduction_pct;
  // one soft grey for every tier: the label carries the level, a rainbow of badges next to every line only adds noise
  const cls = "bg-slate-100 text-slate-600 ring-slate-200";
  if (p >= 40) return { label: "High impact", cls };
  if (p >= 15) return { label: "Medium impact", cls };
  return { label: "Low impact", cls };
}

/** One line under the title: what all open recommendations together are worth, what was verified, what can run itself. */
function PotentialSummary() {
  const t = useT();
  const o = useOptimization().data;
  const cur = useFinopsSummary().data?.currency || "USD";
  const open = useRecommendations("open").data ?? [];
  const jobs = useAutomationJobs().data ?? [];
  if (!o || o.open_count + o.approved_count === 0) return null;
  // From the projects' policies: only types a project explicitly lets the platform apply count as automatic.
  const auto = o.auto_apply_count;
  return (
    <div className="mb-2 rounded-xl bg-sky-50/70 px-3 py-2 text-[11.5px] leading-snug text-sky-900 ring-1 ring-sky-100">
      <p className="text-sm font-semibold text-ink">
        −{formatCarbon(o.potential_carbon_kg_month)} CO₂e/mo
        <span className="ml-2 text-xs font-normal text-slate-600">{o.potential_saving_month > 0 ? t("ov.opt.saving", { amount: formatCost(o.potential_saving_month, cur) }) : t("ov.opt.cost_unknown")}</span>
      </p>
      <p>{t("ov.opt.counts", { count: o.open_count + o.approved_count, unpriced: o.cost_not_estimated_count })}</p>
      <p className="mt-1">
        {o.verified_count === 0 ? t("ov.opt.none_verified") : `${t("ov.opt.verified")}: −${formatCarbon(o.verified_carbon_kg_month)} CO₂e, ${t("ov.opt.saved", { amount: formatCost(o.verified_saving_month, cur) })}`}
      </p>
      <p className="mt-1 flex flex-wrap gap-x-3 text-slate-700">
        <span>{t("ov.opt.auto", { count: auto })}</span>
        <span title={t("ov.opt.approval_hint")}>{t("ov.opt.approval", { count: open.length - auto })} ⓘ</span>
        <Link href="/automation" className="font-semibold text-primary hover:underline">{t("ov.opt.automation")}{jobs.length ? ` · ${jobs.filter((j) => j.status === "planned" || j.status === "approved").length}/${jobs.filter((j) => j.status === "completed").length}` : ""}</Link>
      </p>
    </div>
  );
}

export function TopRecommendations() {
  const t = useT();
  useFinopsSummary(); // re-render when the reporting currency is known
  const q = useRecommendations("open");
  const [more, setMore] = useState(false);
  const sorted = [...(q.data ?? [])].sort((a, b) => b.carbon_reduction_kg_month - a.carbon_reduction_kg_month);
  const top = sorted.slice(0, more ? 8 : 4);
  return (
    <Card index={2} interactive={false} className="p-4">
      <div className="mb-2 flex items-center justify-between gap-3">
        <h2 className="flex items-center gap-2 text-[15px] font-semibold tracking-tight text-ink"><Glyph name="spark" className="size-[18px] text-[#7461d4]" />{t("ov3.recs.title")}<Info>{t("info.recs")}</Info></h2>
        <Link href="/recommendations" className="text-xs font-semibold text-primary hover:underline">{t("ov3.recs.all")}</Link>
      </div>
      <PotentialSummary />
      {q.isLoading ? <Skeleton className="h-40" /> : q.isError ? <ErrorState /> : top.length === 0 ? <div><EmptyState icon={Lightbulb} text={t("ov2.recs.empty")} /><div className="-mt-3 mb-2 text-center"><AllowRegions /></div></div> : (
        <>
          <ul className="space-y-1">
            {top.map((r, i) => {
              const m = ICONS[r.type] ?? FALLBACK, impact = impactOf(r);
              return (
                <li key={r.id} className="rise group" style={{ "--i": i + 3 } as CSSProperties}>
                  <Link href="/recommendations" className="flex items-center gap-2.5 rounded-xl px-1.5 py-1.5 transition-colors hover:bg-slate-50/80">
                    <span className={cn("flex size-8 shrink-0 items-center justify-center rounded-lg transition-transform duration-300 group-hover:scale-110", m.tone)}><m.icon className="size-4" strokeWidth={1.8} /></span>
                    <span className="min-w-0 flex-1">
                      <span className="block truncate text-[13px] font-medium leading-tight" title={r.title}>{r.title}</span>
                      <span className="mt-0.5 flex flex-wrap items-center gap-1">
                        <span className="rounded bg-slate-100 px-1 py-px text-[10px] font-semibold uppercase text-slate-600 ring-1 ring-inset ring-slate-200">{cloudMeta(r.provider).name}</span>
                        <span title={t("ov3.recs.impact_tip")} className={cn("rounded px-1 py-px text-[10px] font-medium ring-1 ring-inset", impact.cls)}>{impact.label}</span>
                        {r.details?.latency_ms && <span title={t("rec.latency_hint", { from: regionName(r.current_region), to: regionName(r.recommended_region), km: r.details.distance_km ?? "?", budget: r.details.latency_budget_ms ?? "?", basis: latencyBasisLabel(t, r.details.latency_basis, r.details.latency_error_pct) })} className="rounded bg-slate-100 px-1 py-px text-[10px] font-medium text-slate-600 ring-1 ring-inset ring-slate-200">{t(r.details.latency_basis === "estimated" || !r.details.latency_basis ? "ov2.reg.latency_chip" : "ov2.reg.latency_chip_measured", { ms: r.details.latency_ms })}</span>}
                      </span>
                    </span>
                    <span className="shrink-0 text-right leading-tight">
                      <span className="flex items-center justify-end gap-1 text-xs font-semibold text-leaf-deep"><Leaf className="size-3" />−{r.carbon_reduction_kg_month.toFixed(1)} kg</span>
                      {r.estimated_cost_impact === null
                        ? <span className="block text-[10.5px] text-muted" title={t(`ov.rec.reason.${r.type}`, undefined, t("ov.rec.reason.other"))}>{t("ov.rec.cost_waiting")}</span>
                        : r.estimated_cost_impact === 0
                          ? <span className="block text-[10.5px] text-muted" title={t("ov.rec.reason.same_price")}>{t("ov.rec.cost_same")}</span>
                          : r.cost_basis === "region_price_list"
                            ? <span className="block text-[10.5px] font-medium text-slate-600" title={t("rec.cost_basis_list")}>{r.estimated_cost_impact < 0 ? "−" : "+"}{t("ov.rec.cost_rough", { amount: formatCost(Math.abs(r.estimated_cost_impact)) })}</span>
                            : <span className="block text-[10.5px] font-medium text-slate-600">{r.estimated_cost_impact < 0 ? "−" : "+"}{formatCost(Math.abs(r.estimated_cost_impact))}/mo</span>}
                    </span>
                  </Link>
                </li>
              );
            })}
          </ul>
          {sorted.length > 4 && <button type="button" onClick={() => setMore((o) => !o)} className="mx-auto mt-1 block text-xs font-semibold text-leaf-deep hover:underline">{more ? t("act.show_less") : t("ov3.recs.more", { n: sorted.length - 4 })}</button>}
        </>
      )}
    </Card>
  );
}

export const Tagline = () => (
  <p className="flex items-center justify-end gap-1.5 pt-1 text-xs text-muted">{content.tagline}<Leaf className="size-3.5 text-leaf-deep" /></p>
);
