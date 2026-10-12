"use client";

import { Card } from "@/components/ui/card";
import { CountUp } from "@/components/ui/count-up";
import { IconCheck, IconClock, IconReport, IconWarning } from "@/components/ui/icons";
import { Info } from "@/components/ui/info";
import { ReportBuilder } from "@/components/reports/report-builder";
import { ReportSchedules } from "@/components/reports/report-schedules";
import { ReportList } from "@/components/reports/report-list";
import { useReports } from "@/hooks/use-overview";
import { useT } from "@/i18n/provider";

function Stat({ index, icon: Icon, tone, label, value, info }: { index: number; icon: typeof IconReport; tone: string; label: string; value: number; info: string }) {
  return (
    <Card index={index} className="flex items-center gap-4 p-4">
      <Icon className={`size-11 shrink-0 ${tone}`} strokeWidth={1.4} />
      <div>
        <p className="flex items-center gap-1 text-[13px] font-medium text-slate-600">{label}<Info>{info}</Info></p>
        <p className="mt-0.5 text-2xl font-bold tracking-tight tabular-nums"><CountUp value={value} format={(n) => String(Math.round(n))} /></p>
      </div>
    </Card>
  );
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
        <Stat index={0} icon={IconReport} tone="text-leaf-deep" label={t("rp.stat.total")} value={all.length} info={t("rp.info.total")} />
        <Stat index={1} icon={IconCheck} tone="text-[#5b8fdc]" label={t("rp.stat.ready")} value={count("ready")} info={t("rp.info.ready")} />
        <Stat index={2} icon={IconClock} tone="text-[#d9a441]" label={t("rp.stat.pending")} value={count("pending")} info={t("rp.info.pending")} />
        <Stat index={3} icon={IconWarning} tone="text-[#d9707f]" label={t("rp.stat.failed")} value={count("failed")} info={t("rp.info.failed")} />
      </div>
      {canCreate && <ReportBuilder onRequested={() => undefined} />}
      <ReportSchedules canAct={canCreate} />
      <ReportList canAct={canCreate} />
    </div>
  );
}
