"use client";

import Link from "next/link";
import { CarbonBreakdown, CarbonKpis, CarbonTrend, GridHistory, IntensityOrigin, ScheduleAdvisor } from "@/components/carbon/carbon-board";
import { SciCalculator } from "@/components/carbon/sci-calculator";
import { Badge } from "@/components/ui/badge";
import { Card, CardHeader } from "@/components/ui/card";
import { EmptyState } from "@/components/ui/empty-state";
import { IconLeaf, IconReport } from "@/components/ui/icons";
import { Skeleton } from "@/components/ui/skeleton";
import { useCarbonSummary, useMethodologyVersions } from "@/hooks/use-overview";
import { useT } from "@/i18n/provider";

export function CarbonView() {
  const t = useT();
  const sum = useCarbonSummary();
  const versions = useMethodologyVersions();
  const m = sum.data?.methodology;
  const empty = !!sum.data && !sum.data.has_data;
  return (
    <div className="page-enter space-y-6">
      <div className="flex flex-wrap items-end justify-between gap-4">
        <div>
          <h1 className="text-2xl font-bold tracking-tight">{t("carbon.title")}</h1>
          <p className="mt-1 max-w-2xl text-sm text-muted">{t("cb.subtitle")}</p>
        </div>
        {m && (
          <Link href="/methodology" className="group inline-flex items-center gap-2 rounded-full border border-line bg-white px-3 py-1.5 text-xs font-medium text-slate-600 transition-all hover:border-leaf/50 hover:text-ink">
            <span className={`size-2 rounded-full ${m.status === "verified" ? "bg-leaf" : "bg-pastel-amber"}`} />
            {t("cb.method.chip", { version: m.version })}
            <span className="text-muted">· {m.status}</span>
          </Link>
        )}
      </div>

      {empty ? (
        <Card interactive={false}><EmptyState icon={IconLeaf} text={t("cb.empty")} action={{ href: "/connectors", label: t("ov2.empty.connect") }} /></Card>
      ) : (
        <>
          <CarbonKpis />
          <div className="grid gap-5 xl:grid-cols-[minmax(0,1.35fr)_minmax(0,1fr)]">
            <CarbonTrend />
            <CarbonBreakdown />
          </div>
          <GridHistory />
          <div className="grid gap-5 xl:grid-cols-2">
            <ScheduleAdvisor />
            {sum.data?.has_data ? <SciCalculator summary={sum.data} /> : <Skeleton className="h-72" />}
          </div>
          <IntensityOrigin />
          <Card index={9} interactive={false}>
            <CardHeader icon={<IconReport className="size-[18px] text-[#8a78d8]" />} title={t("carbon.methodology")} info={t("info.carbon_method")}
              action={m && <Badge tone={m.status === "verified" ? "good" : "warn"}>{m.status}</Badge>} />
            {!m ? <Skeleton className="h-24" /> : (
              <div className="grid gap-5 text-sm md:grid-cols-[minmax(0,1fr)_minmax(0,1.2fr)]">
                <div className="space-y-2">
                  <p><b>{m.version}</b> — {m.provenance}</p>
                  <p className="text-muted">{t("cb.method.embodied")}: <b className="text-ink">{m.embodied_carbon_included ? t("cb.method.included") : t("cb.method.not_included")}</b></p>
                  {versions.data && versions.data.length > 0 && (
                    <p className="text-xs text-muted">{t("cb.method.versions")}: {versions.data.map((v) => `${v.version}${v.active ? ` (${t("cb.method.active")})` : ""}`).join(", ")}</p>
                  )}
                  <Link href="/methodology" className="inline-block text-sm font-medium text-leaf-deep hover:underline">{t("cb.method.read")} →</Link>
                </div>
                <ul className="space-y-1.5 text-muted">
                  {m.caveats.map((c) => <li key={c} className="flex gap-2"><span className="mt-2 size-1.5 shrink-0 rounded-full bg-slate-300" />{c}</li>)}
                </ul>
              </div>
            )}
          </Card>
        </>
      )}
    </div>
  );
}
