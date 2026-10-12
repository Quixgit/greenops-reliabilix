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
    <section className="rise relative overflow-hidden rounded-2xl bg-gradient-to-r from-leaf-soft via-white to-white py-2.5 pl-4 pr-11 ring-1 ring-leaf/25" aria-label={t("ff.title")}>
      <button type="button" aria-label={t("ff.close")} title={t("ff.close")} onClick={() => { setHidden(true); try { window.localStorage.setItem(KEY, "1"); } catch { /* the choice just does not persist */ } }}
        className="absolute right-2 top-2 grid size-7 place-items-center rounded-full text-slate-400 transition-colors hover:bg-white hover:text-ink"><IconClose className="size-3.5" /></button>
      <div className="flex flex-wrap items-center gap-x-5 gap-y-2">
        <p className="flex items-center gap-1.5 text-xs font-semibold uppercase tracking-wide text-leaf-deep"><IconSparkles className="size-4" />{t("ff.title")}</p>
        <dl className="flex flex-wrap items-center gap-x-5 gap-y-1">
          {stats.map((s, i) => (
            <div key={s.label} className="rise flex items-baseline gap-1.5" style={{ "--i": i + 1 } as CSSProperties} title={s.hint}>
              <dd className="text-base font-bold tracking-tight text-ink tabular-nums">{s.node}</dd>
              <dt className="text-xs text-slate-500">{s.label}</dt>
            </div>
          ))}
        </dl>
        {top.length > 0 && (
          <ul className="flex flex-wrap items-center gap-1.5">
            {top.slice(0, 2).map((r) => (
              <li key={r.id}>
                <Link href="/recommendations" className="group inline-flex max-w-[16rem] items-center gap-1.5 rounded-full bg-white px-2.5 py-1 text-xs ring-1 ring-line transition-all hover:ring-leaf/60" title={r.title}>
                  <span className="truncate font-medium text-ink">{r.title}</span>
                  <span className="shrink-0 font-semibold text-leaf-deep">−{formatCost(Math.abs(r.estimated_cost_impact ?? 0), cur)}{t("ff.per_month")}</span>
                  <IconArrowRight className="size-3 shrink-0 text-slate-400 transition-transform duration-300 group-hover:translate-x-0.5" />
                </Link>
              </li>
            ))}
          </ul>
        )}
      </div>
    </section>
  );
}
