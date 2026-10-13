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
import { PageHead, Sections } from "@/components/dx/page";
import { CloudSwitcher } from "@/components/dashboard/cloud-switcher";
import { useT } from "@/i18n/provider";

export function CarbonView() {
  const t = useT();
  const sum = useCarbonSummary();
  const versions = useMethodologyVersions();
  const m = sum.data?.methodology;
  const empty = !!sum.data && !sum.data.has_data;
  return (
    <div className="page-enter">
      <PageHead title={t("carbon.title")} sub={t("cb.subtitle")} actions={<>
        <CloudSwitcher />
        {m && (
          <Link href="/methodology" className="dx-btn is-sm" title={t("cb.method.read")}>
            <span className="dx-swatch" style={{ background: m.status === "verified" ? "var(--dx-good)" : "var(--dx-butter-ink)" }} />
            {t("cb.method.chip", { version: m.version })} · {m.status}
          </Link>
        )}
      </>} />

      {empty ? (
        <Card interactive={false}><EmptyState icon={IconLeaf} text={t("cb.empty")} action={{ href: "/connectors", label: t("ov2.empty.connect") }} /></Card>
      ) : (
        <Sections label={t("carbon.title")} tabs={[
          { id: "emissions", label: t("dx.cb.emissions", undefined, "Emissions") },
          { id: "grid", label: t("dx.cb.grid", undefined, "Grid") },
          { id: "plan", label: t("dx.cb.plan", undefined, "Plan and SCI") },
          { id: "method", label: t("dx.cb.method", undefined, "Method") },
        ]}>
          {(tab) => tab === "emissions" ? (
            <>
              <CarbonKpis />
              <div className="dxp-split"><CarbonTrend /><CarbonBreakdown /></div>
            </>
          ) : tab === "grid" ? (
            <>
              <GridHistory />
              <IntensityOrigin />
            </>
          ) : tab === "plan" ? (
            <div className="dxp-split is-even">
              <ScheduleAdvisor />
              {sum.data?.has_data ? <SciCalculator summary={sum.data} /> : <Skeleton className="h-72" />}
            </div>
          ) : (
          <Card index={9} interactive={false}>
            <CardHeader icon={<IconReport className="size-[18px] text-[#5a6aa0]" />} title={t("carbon.methodology")} info={t("info.carbon_method")}
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
          )}
        </Sections>
      )}
    </div>
  );
}
