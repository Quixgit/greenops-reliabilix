"use client";

import { Tile } from "@/components/dashboard/tile";
import type { GlyphName, Tone } from "@/components/ui/glyphs";
import { CountUp } from "@/components/ui/count-up";
import { ReportBuilder } from "@/components/reports/report-builder";
import { ReportSchedules } from "@/components/reports/report-schedules";
import { ReportList } from "@/components/reports/report-list";
import { useReports } from "@/hooks/use-overview";
import { useT } from "@/i18n/provider";

function Stat({ index, glyph, tone, label, value, info }: { index: number; glyph: GlyphName; tone: Tone; label: string; value: number; info: string }) {
  return <Tile index={index} icon={glyph} tone={tone} label={label} info={info} value={<CountUp value={value} format={(n) => String(Math.round(n))} />} />;
}

export function ReportsView({ canCreate }: { canCreate: boolean }) {
  const t = useT();
  const all = useReports().data ?? [];
  const count = (s: string) => all.filter((r) => r.status === s).length;
  return (
    <div className="page-enter space-y-6">
      <div>
        <h1 className="text-2xl font-bold tracking-tight">{t("rp.title")}</h1>
        <p className="mt-1 max-w-2xl text-sm text-muted">{t("rp.subtitle")}</p>
      </div>
      <div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-4">
        <Stat index={0} glyph="list" tone="green" label={t("rp.stat.total")} value={all.length} info={t("rp.info.total")} />
        <Stat index={1} glyph="unit" tone="blue" label={t("rp.stat.ready")} value={count("ready")} info={t("rp.info.ready")} />
        <Stat index={2} glyph="calendar" tone="amber" label={t("rp.stat.pending")} value={count("pending")} info={t("rp.info.pending")} />
        <Stat index={3} glyph="pulse" tone="rose" label={t("rp.stat.failed")} value={count("failed")} info={t("rp.info.failed")} />
      </div>
      {canCreate && <ReportBuilder onRequested={() => undefined} />}
      <ReportSchedules canAct={canCreate} />
      <ReportList canAct={canCreate} />
    </div>
  );
}
