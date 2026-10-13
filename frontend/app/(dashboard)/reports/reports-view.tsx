"use client";

import { PageHead, Sections } from "@/components/dx/page";
import { ReportBuilder } from "@/components/reports/report-builder";
import { ReportSchedules } from "@/components/reports/report-schedules";
import { ReportList } from "@/components/reports/report-list";
import { useReports } from "@/hooks/use-overview";
import { useT } from "@/i18n/provider";

export function ReportsView({ canCreate }: { canCreate: boolean }) {
  const t = useT();
  const all = useReports().data ?? [];
  const failed = all.filter((r) => r.status === "failed").length;
  return (
    <div className="page-enter">
      <PageHead title={t("rp.title")} sub={t("rp.subtitle")} />
      <Sections label={t("rp.title")} initial={canCreate ? "new" : "files"} tabs={[
        { id: "new", label: t("dx.rp.new", undefined, "New report"), hidden: !canCreate },
        { id: "files", label: t("dx.rp.files", undefined, "Reports"), count: all.length },
        { id: "schedules", label: t("dx.rp.schedules", undefined, "Schedules") },
      ]}>
        {(tab) => tab === "new" ? <ReportBuilder onRequested={() => undefined} /> : tab === "files" ? (
          <>
            {failed > 0 && <p className="dx-note is-rose">{t("dx.rp.failed", { n: failed }, `${failed} report(s) failed. Open the row to see why and run it again.`)}</p>}
            <ReportList canAct={canCreate} />
          </>
        ) : <ReportSchedules canAct={canCreate} />}
      </Sections>
    </div>
  );
}
