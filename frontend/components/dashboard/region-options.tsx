"use client";

import { isMeasured, latencyBasisLabel } from "@/lib/latency-basis";
import { useState } from "react";
import { IconChevronDown } from "@/components/ui/icons";
import { useT } from "@/i18n/provider";
import { formatCost, formatCarbon } from "@/lib/formatters";
import { regionName } from "@/lib/regions";
import { cn } from "@/lib/utils";

export type RegionOption = {
  region: string; g_per_kwh: number; carbon_reduction_pct: number; carbon_reduction_kg_month: number; latency_ms?: number; distance_km?: number; latency_basis?: string; latency_error_pct?: number;
  cost_change_pct?: number; within_latency_budget: boolean; chosen?: boolean; tags?: string[];
};

/** The places a workload could go, as the engine weighed them. Strings only travel in `details`, so the list is JSON in a string. */
export function parseOptions(details: Record<string, string> | null | undefined): RegionOption[] {
  try {
    const v = JSON.parse(details?.options ?? "[]") as RegionOption[];
    return Array.isArray(v) ? v : [];
  } catch {
    return [];
  }
}

/** Three facts a move changes at once: how much carbon, how much it costs and how far the new region is. */
export function Metric({ label, children, tone, title }: { label: string; children: React.ReactNode; tone: "leaf" | "sky" | "amber" | "slate" | "rose"; title?: string }) {
  const c = { leaf: "bg-leaf-soft text-leaf-deep ring-leaf/20", sky: "bg-sky-50 text-[#3f78b8] ring-sky-100", amber: "bg-amber-50 text-[#8a6218] ring-amber-100", slate: "bg-slate-50 text-slate-600 ring-line", rose: "bg-rose-50 text-[#b9566a] ring-rose-100" }[tone];
  return (
    <span title={title} className={cn("inline-flex items-center gap-1.5 rounded-full px-2.5 py-1 text-xs ring-1", c)}>
      <span className="font-medium opacity-70">{label}</span><span className="font-semibold">{children}</span>
    </span>
  );
}

/** Latency of a proposed move: "≈ 12 ms away", with how it was got in the tooltip. */
export function LatencyChip({ details, from, to }: { details: Record<string, string> | null | undefined; from: string; to: string }) {
  const t = useT();
  const ms = details?.latency_ms;
  if (!ms) return <Metric label={t("rec.latency")} tone="slate" title={t("rec.latency_unknown")}>{t("rec.latency_unknown")}</Metric>;
  return (
    <Metric label={t("rec.latency")} tone="sky" title={t("rec.latency_hint", { from: regionName(from), to: regionName(to), km: details?.distance_km ?? "?", budget: details?.latency_budget_ms ?? "?", basis: latencyBasisLabel(t, details?.latency_basis, details?.latency_error_pct) })}>
      {details?.latency_basis === "estimated" || !details?.latency_basis ? t("rec.latency_add", { ms }) : t("rec.latency_add_measured", { ms })}
    </Metric>
  );
}

/** The options side by side, the proposed one marked and those beyond the latency limit shown for what they are. */
export function CompareOptions({ options, budget }: { options: RegionOption[]; budget: string | undefined }) {
  const t = useT();
  const [open, setOpen] = useState(false);
  if (options.length < 2) return null;
  return (
    <div>
      <button type="button" aria-expanded={open} onClick={() => setOpen((o) => !o)} className="inline-flex items-center gap-1.5 text-xs font-semibold text-leaf-deep hover:underline">
        {open ? t("rec.compare_hide") : t("rec.compare", { n: options.length })}
        <IconChevronDown className={cn("size-3.5 transition-transform duration-300", open && "rotate-180")} aria-hidden />
      </button>
      <div className={cn("grid transition-[grid-template-rows] duration-500 ease-[var(--ease-out-soft)]", open ? "grid-rows-[1fr]" : "grid-rows-[0fr]")}>
        <div className="overflow-hidden">
          <div className="mt-2 overflow-x-auto rounded-xl ring-1 ring-line">
            <table className="w-full min-w-[480px] text-xs">
              <thead className="bg-slate-50/70 text-left text-[11px] uppercase tracking-wide text-slate-500">
                <tr><th className="px-3 py-2 font-semibold">{t("rec.col.region")}</th><th className="px-3 py-2 font-semibold">{t("rec.col.carbon")}</th><th className="px-3 py-2 font-semibold">{t("rec.col.latency")}</th><th className="px-3 py-2 font-semibold">{t("rec.col.cost")}</th></tr>
              </thead>
              <tbody>
                {options.map((o) => (
                  <tr key={o.region} className={cn("border-t border-line/60", o.chosen && "bg-leaf-soft/40", !o.within_latency_budget && "text-slate-400")}>
                    <td className="px-3 py-2"><span className="font-medium text-ink">{regionName(o.region)}</span>
                      <span className="ml-1.5 inline-flex flex-wrap gap-1 align-middle">
                        {o.chosen && <span className="rounded-full bg-leaf-deep px-1.5 py-px text-[10px] font-semibold text-white">{t("rec.opt.chosen")}</span>}
                        {o.tags?.map((g) => <span key={g} className="rounded-full bg-white px-1.5 py-px text-[10px] font-semibold text-slate-600 ring-1 ring-line">{t(`rec.tag.${g}`)}</span>)}
                        {!o.within_latency_budget && <span className="rounded-full bg-rose-50 px-1.5 py-px text-[10px] font-semibold text-[#b9566a]">{t("rec.opt.far", { budget: budget ?? "" })}</span>}
                      </span></td>
                    <td className="px-3 py-2 tabular-nums">−{Math.round(o.carbon_reduction_pct)}% <span className="text-slate-400">({formatCarbon(o.carbon_reduction_kg_month)}/mo)</span></td>
                    <td className="px-3 py-2 tabular-nums" title={latencyBasisLabel(t, o.latency_basis, o.latency_error_pct)}>{o.latency_ms != null ? `${o.latency_basis === "estimated" || !o.latency_basis ? "≈ " : ""}${o.latency_ms} ms` : "—"}{o.latency_ms != null && o.latency_basis && <span className={cn("ml-1 text-[10px]", isMeasured(o.latency_basis) ? "text-leaf-deep" : "text-slate-400")}>{isMeasured(o.latency_basis) ? t("lat.measured_tag") : `±${o.latency_error_pct ?? 0}%`}</span>}</td>
                    <td className="px-3 py-2 tabular-nums">{o.cost_change_pct != null ? `${o.cost_change_pct > 0 ? "+" : ""}${o.cost_change_pct}%` : "—"}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
          <p className="mt-2 text-[11px] text-muted">{t("rec.compare_note", { budget: budget ?? "" })}</p>
        </div>
      </div>
    </div>
  );
}

export { formatCost };
