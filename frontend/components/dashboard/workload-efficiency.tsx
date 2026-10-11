"use client";

import { Gauge } from "lucide-react";
import Link from "next/link";
import { Card, CardHeader } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { useEfficiency, useFinopsSummary } from "@/hooks/use-overview";
import { useT } from "@/i18n/provider";
import { formatCost, nf } from "@/lib/formatters";

/** Cost and CO2e per unit of work, only over the days the customer reported units for; the unit is always named. */
export function WorkloadEfficiency() {
  const t = useT();
  const q = useEfficiency();
  const cur = useFinopsSummary().data?.currency || "USD";
  const e = q.data;
  const unit = e?.unit ? e.unit.replace(/_/g, " ") : "";
  return (
    <Card index={4} interactive={false}>
      <CardHeader title={t("ov.eff.title")} icon={<Gauge className="size-4 text-violet-500" />} />
      {q.isLoading ? <Skeleton className="h-24" /> : !e || !e.has_units ? (
        <div className="space-y-3">
          <p className="text-sm leading-relaxed text-muted">{t("ov.eff.no_units")}</p>
          <Link href="/projects" className="inline-flex items-center gap-1 text-sm font-semibold text-primary hover:underline">{t("ov2.eff.report")} →</Link>
        </div>
      ) : e.mixed_units ? (
        <p className="text-sm leading-relaxed text-muted">{t("ov.eff.mixed")}</p>
      ) : (
        <div className="grid grid-cols-2 gap-4">
          <div className="rounded-2xl bg-violet-50/70 p-3.5 ring-1 ring-violet-100">
            <p className="text-xs font-medium text-violet-800">{t("ov.eff.co2_per", { unit })}</p>
            <p className="mt-1 text-xl font-bold tabular-nums">{nf(e.carbon_g_per_unit! < 10 ? 2 : 0).format(e.carbon_g_per_unit!)} <span className="text-xs font-medium text-muted">g</span></p>
          </div>
          <div className="rounded-2xl bg-sky-50/70 p-3.5 ring-1 ring-sky-100">
            <p className="text-xs font-medium text-sky-800">{t("ov.eff.cost_per", { unit })}</p>
            <p className="mt-1 text-xl font-bold tabular-nums">{e.cost_per_unit! < 1 ? `${nf(4).format(e.cost_per_unit!)} ${cur}` : formatCost(e.cost_per_unit!, cur)}</p>
          </div>
          <p className="col-span-2 text-[11px] leading-snug text-muted">{t("ov.eff.basis", { units: nf(0).format(e.units), unit, days: e.days })}</p>
        </div>
      )}
    </Card>
  );
}
