"use client";

import Link from "next/link";
import { useState, type CSSProperties } from "react";
import { CountUp } from "@/components/ui/count-up";
import { IconArrowRight, IconClose, IconSparkles } from "@/components/ui/icons";
import { useCommitments } from "@/hooks/use-commitments";
import { useConnections, useFinopsSummary, useOptimization, useRecommendations } from "@/hooks/use-overview";
import { useT } from "@/i18n/provider";
import { formatCarbon, formatCost } from "@/lib/formatters";

const KEY = "rlx.findings.hidden";
const stored = () => { try { return window.localStorage.getItem(KEY) === "1"; } catch { return false; } };

/**
 * What the platform found in the first sync, in one place at the top: the money and carbon the open recommendations are worth, the
 * commitment saving, and the three best moves. It is the answer to "what did I get for connecting?" and can be closed.
 */
export function FirstFindings() {
  const t = useT();
  const [hidden, setHidden] = useState(() => (typeof window === "undefined" ? false : stored()));
  const conns = useConnections().data ?? [];
  const o = useOptimization().data;
  const cur = useFinopsSummary().data?.currency || "USD";
  const open = useRecommendations("open").data ?? [];
  const commit = useCommitments({ lookback: 60, term: 1, coverage: 70, discounts: {} }).data;
  const synced = conns.some((c) => !!c.last_sync_at);
  if (hidden || !synced || !o || o.open_count + o.approved_count === 0) return null;

  const top = [...open].filter((r) => r.estimated_cost_impact !== null && r.estimated_cost_impact < 0 && r.cost_basis !== "region_price_list").sort((a, b) => (a.estimated_cost_impact ?? 0) - (b.estimated_cost_impact ?? 0)).slice(0, 3);
  const stats: { label: string; node: React.ReactNode; hint: string }[] = [
    { label: t("ff.saving"), hint: t("ff.saving_hint"), node: <CountUp value={o.potential_saving_month * 12} format={(n) => formatCost(n, cur)} /> },
    { label: t("ff.carbon"), hint: t("ff.carbon_hint"), node: <CountUp value={o.potential_carbon_kg_month * 12} format={(n) => formatCarbon(n)} /> },
    { label: t("ff.moves"), hint: t("ff.moves_hint", { unpriced: o.cost_not_estimated_count }), node: <CountUp value={o.open_count + o.approved_count} format={(n) => String(Math.round(n))} /> },
  ];
  if (commit && commit.total_annual_saving > 0) stats.push({ label: t("ff.commit"), hint: t("ff.commit_hint"), node: <CountUp value={commit.total_annual_saving} format={(n) => formatCost(n, commit.currency)} /> });
  return (
    <section className="rise relative overflow-hidden rounded-3xl bg-gradient-to-br from-leaf-soft via-white to-white p-5 ring-1 ring-leaf/30 sm:p-6" aria-label={t("ff.title")}>
      <button type="button" aria-label={t("ff.close")} title={t("ff.close")} onClick={() => { setHidden(true); try { window.localStorage.setItem(KEY, "1"); } catch { /* the choice just does not persist */ } }}
        className="absolute right-3 top-3 grid size-8 place-items-center rounded-full text-slate-400 transition-colors hover:bg-white hover:text-ink"><IconClose className="size-4" /></button>
      <p className="flex items-center gap-2 text-[13px] font-semibold uppercase tracking-wide text-leaf-deep"><IconSparkles className="size-4" />{t("ff.title")}</p>
      <h2 className="mt-1 max-w-2xl text-xl font-bold tracking-tight text-ink sm:text-2xl">{t("ff.lead")}</h2>
      <dl className="mt-4 grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
        {stats.map((s, i) => (
          <div key={s.label} className="rise rounded-2xl bg-white/80 px-4 py-3 ring-1 ring-line/70" style={{ "--i": i + 1 } as CSSProperties} title={s.hint}>
            <dt className="text-xs font-medium text-slate-500">{s.label}</dt>
            <dd className="mt-0.5 text-2xl font-bold tracking-tight text-ink tabular-nums">{s.node}</dd>
          </div>
        ))}
      </dl>
      {top.length > 0 && (
        <div className="mt-4">
          <p className="mb-1.5 text-xs font-semibold uppercase tracking-wide text-slate-500">{t("ff.start_with")}</p>
          <ul className="grid gap-2 lg:grid-cols-3">
            {top.map((r, i) => (
              <li key={r.id} className="rise" style={{ "--i": i + 5 } as CSSProperties}>
                <Link href="/recommendations" className="group flex h-full items-center justify-between gap-3 rounded-xl bg-white px-3.5 py-2.5 text-sm ring-1 ring-line transition-all duration-300 hover:-translate-y-0.5 hover:ring-leaf/60">
                  <span className="min-w-0"><span className="block truncate font-medium text-ink">{r.title}</span><span className="text-xs font-semibold text-leaf-deep">−{formatCost(Math.abs(r.estimated_cost_impact ?? 0), cur)}{t("ff.per_month")} · −{r.carbon_reduction_kg_month.toFixed(1)} kg CO₂e</span></span>
                  <IconArrowRight className="size-4 shrink-0 text-slate-400 transition-transform duration-300 group-hover:translate-x-1" />
                </Link>
              </li>
            ))}
          </ul>
        </div>
      )}
      <p className="mt-3 text-xs text-muted">{t("ff.note")}</p>
    </section>
  );
}
