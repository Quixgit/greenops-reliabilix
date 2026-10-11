"use client";

import { Calculator } from "lucide-react";
import Link from "next/link";
import { useState } from "react";
import { Card, CardHeader } from "@/components/ui/card";
import { useT } from "@/i18n/provider";
import type { CarbonSummary } from "@/lib/api/schemas";
import { nf } from "@/lib/formatters";
import { changePct, computeSci, formatGrams } from "@/lib/sci";

const input = "w-full rounded-lg bg-white px-3 py-2 text-sm ring-1 ring-line focus:outline-none focus:ring-2 focus:ring-primary";

/** Reads a positive number the way people type it ("1 200 000", "1,5"); null for anything else. */
function parseUnits(s: string): number | null {
  const v = Number(s.replace(/[\s ]/g, "").replace(",", "."));
  return Number.isFinite(v) && v > 0 ? v : null;
}

function grams(g: number) {
  const f = formatGrams(g);
  return `${nf(f.value >= 100 ? 0 : f.value >= 10 ? 1 : 2).format(f.value)} ${f.unit}`;
}

/**
 * SCI = ((E x I) + M) / R for the period on screen. E, I and M come from the period's own totals; R is the one input
 * the platform cannot know (what a "unit" of the application is), so the person says it here.
 */
export function SciCalculator({ summary }: { summary: CarbonSummary }) {
  const t = useT();
  const [unit, setUnit] = useState("");
  const [units, setUnits] = useState("");
  const [prevUnits, setPrevUnits] = useState("");

  const embodiedIncluded = summary.methodology.embodied_carbon_included;
  const r = parseUnits(units);
  const now = r === null ? null : computeSci({ energyKwh: summary.energy_kwh, carbonKg: summary.carbon_kg_co2e, embodiedIncluded, units: r });
  const rPrev = parseUnits(prevUnits);
  const before = rPrev === null || !summary.comparable ? null : computeSci({ energyKwh: summary.previous.energy_kwh, carbonKg: summary.previous.carbon_kg_co2e, embodiedIncluded, units: rPrev });
  const delta = now && before ? changePct(now.perUnitG, before.perUnitG) : null;
  const name = unit.trim() || t("carbon.sci.unit_fallback");

  return (
    <Card index={1}>
      <CardHeader title={t("carbon.sci.title")} action={<Calculator className="size-4 text-muted" aria-hidden />} />
      <p className="mb-4 text-sm text-muted">{t("carbon.sci.intro")}</p>
      <div className="grid gap-4 md:grid-cols-2">
        <div className="space-y-3">
          <label className="block text-sm">
            <span className="mb-1 block font-medium text-slate-700">{t("carbon.sci.unit_label")}</span>
            <input value={unit} onChange={(e) => setUnit(e.target.value)} maxLength={40} placeholder={t("carbon.sci.unit_placeholder")} className={input} />
          </label>
          <label className="block text-sm">
            <span className="mb-1 block font-medium text-slate-700">{t("carbon.sci.units_label")}</span>
            <input value={units} onChange={(e) => setUnits(e.target.value)} inputMode="decimal" placeholder="1 000 000" className={input} aria-invalid={units !== "" && r === null} />
            {units !== "" && r === null && <span role="alert" className="mt-1 block text-xs text-bad">{t("carbon.sci.units_invalid")}</span>}
          </label>
          {summary.comparable && (
            <label className="block text-sm">
              <span className="mb-1 block font-medium text-slate-700">{t("carbon.sci.prev_label")}</span>
              <input value={prevUnits} onChange={(e) => setPrevUnits(e.target.value)} inputMode="decimal" placeholder="1 000 000" className={input} aria-invalid={prevUnits !== "" && rPrev === null} />
            </label>
          )}
          <p className="text-xs text-muted">{t("carbon.sci.reported_hint")} <Link href="/projects" className="font-medium text-primary hover:underline">{t("carbon.sci.open_projects")}</Link></p>
        </div>

        <div className="rounded-xl bg-leaf-soft px-4 py-4 ring-1 ring-leaf-soft" aria-live="polite">
          <p className="font-mono text-xs text-leaf-deep/80">SCI = ((E × I) + M) / R</p>
          {now ? (
            <>
              <p className="mt-2 text-3xl font-bold tracking-tight text-ink">{grams(now.perUnitG)}<span className="ml-1 text-sm font-medium text-slate-600">{t("carbon.sci.per", { unit: name })}</span></p>
              {delta !== null && (
                <p className="mt-1 text-sm text-leaf-deep">{t("carbon.sci.change", { pct: `${delta > 0 ? "+" : ""}${nf(1).format(delta)}%` })}</p>
              )}
              <dl className="mt-3 grid grid-cols-[auto_1fr] gap-x-3 gap-y-1 text-xs">
                <dt className="text-muted">E</dt><dd className="tabular-nums">{nf(1).format(summary.energy_kwh)} kWh</dd>
                <dt className="text-muted">I</dt><dd className="tabular-nums">{nf(0).format(now.intensityGPerKwh)} gCO₂e/kWh</dd>
                <dt className="text-muted">M</dt><dd>{embodiedIncluded ? t("carbon.sci.m_in_total") : t("carbon.sci.m_not_included")}</dd>
                <dt className="text-muted">R</dt><dd className="tabular-nums">{nf(0).format(r!)} {name}</dd>
              </dl>
            </>
          ) : (
            <p className="mt-2 text-sm text-slate-600">{t("carbon.sci.enter_units")}</p>
          )}
        </div>
      </div>
      <div className="mt-4 rounded-xl bg-slate-50 px-4 py-3 text-xs leading-relaxed text-slate-700 ring-1 ring-line">
        <p className="font-semibold">{t("carbon.sci.limits_title")}</p>
        <ul className="mt-1 list-disc space-y-1 pl-5">
          <li>{t("carbon.sci.limit_boundary")}</li>
          <li>{embodiedIncluded ? t("carbon.sci.limit_embodied_in") : t("carbon.sci.limit_embodied_out")}</li>
          <li>{t("carbon.sci.limit_average")}</li>
          <li>{t("carbon.sci.limit_unit")}</li>
        </ul>
      </div>
    </Card>
  );
}
