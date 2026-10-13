"use client";

import Link from "next/link";
import { Card } from "@/components/ui/card";
import { Glyph } from "@/components/ui/glyphs";
import { Info } from "@/components/ui/info";
import { Skeleton } from "@/components/ui/skeleton";
import { useEfficiency, useFinopsSummary } from "@/hooks/use-overview";
import { useT } from "@/i18n/provider";
import { formatCostFine, nf } from "@/lib/formatters";

const compact = (n: number) => new Intl.NumberFormat(undefined, { notation: "compact", maximumFractionDigits: 1 }).format(n);
const fmtG = (g: number) => (g >= 1000 ? `${nf(g >= 10_000 ? 0 : 1).format(g / 1000)} kg` : `${nf(g < 10 ? 2 : g < 100 ? 1 : 0).format(g)} g`);
/** The smallest batch of units (1, 1,000, 1,000,000) in which the cost is at least a cent: "0.0001 per request" is unreadable, "0.10 per 1,000" is not. */
function batch(costPerUnit: number): number {
  for (const n of [1, 1_000, 1_000_000]) if (costPerUnit * n >= 0.01) return n;
  return 1_000_000;
}

/** Cost and CO2e per unit of work, only over the days the customer reported units for; the unit is always named. */
export function WorkloadEfficiency() {
  const t = useT();
  const q = useEfficiency();
  const cur = useFinopsSummary().data?.currency || "USD";
  const e = q.data;
  const unit = e?.unit ? e.unit.replace(/_/g, " ") : "";
  const n = e?.cost_per_unit ? batch(e.cost_per_unit) : 1;
  const scale = { n, label: n === 1 ? unit : `${n === 1000 ? "1,000" : "1,000,000"} ${unit.endsWith("s") ? unit : `${unit}s`}` };
  return (
    <Card index={4} interactive={false} className="p-4">
      <h2 className="mb-3 flex items-center gap-2 text-[15px] font-semibold tracking-tight text-ink"><Glyph name="gauge" className="size-[18px] text-[#5a6aa0]" />{t("ov.eff.title")}</h2><Info corner>{t("info.efficiency")}</Info>
      {q.isLoading ? <Skeleton className="h-24" /> : !e || !e.has_units ? (
        <div className="space-y-3">
          <p className="text-sm leading-relaxed text-muted">{t("ov.eff.no_units")}</p>
          <Link href="/projects" className="inline-flex items-center gap-1 text-sm font-semibold text-primary hover:underline">{t("ov2.eff.report")} →</Link>
        </div>
      ) : e.mixed_units ? (
        <p className="text-sm leading-relaxed text-muted">{t("ov.eff.mixed")}</p>
      ) : (
        <div className="grid grid-cols-2 gap-4">
          <div className="rounded-xl bg-violet-50/70 p-3 ring-1 ring-violet-100">
            <p className="text-xs font-medium text-violet-800">{t("ov.eff.co2_per", { unit: scale.label })}</p>
            <p className="mt-1 text-xl font-bold tabular-nums">{fmtG(e.carbon_g_per_unit! * scale.n)}</p>
          </div>
          <div className="rounded-xl bg-sky-50/70 p-3 ring-1 ring-sky-100">
            <p className="text-xs font-medium text-sky-800">{t("ov.eff.cost_per", { unit: scale.label })}</p>
            <p className="mt-1 text-xl font-bold tabular-nums">{formatCostFine(e.cost_per_unit! * scale.n, cur)}</p>
          </div>
          <p className="col-span-2 text-[11px] leading-snug text-muted">{t("ov3.eff.basis", { units: compact(e.units), unit: unit || "units", days: e.days })}</p>
        </div>
      )}
    </Card>
  );
}
