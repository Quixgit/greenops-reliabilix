import { CloudCosts } from "@/components/dashboard/cloud-costs";
import { ConnectedAccounts } from "@/components/dashboard/connected-accounts";
import { DataQuality } from "@/components/dashboard/data-quality";
import { DemoBanner } from "@/components/dashboard/demo-banner";
import { FirstFindings } from "@/components/dashboard/first-findings";
import { FinopsStrip } from "@/components/dashboard/finops-strip";
import { FootprintSplit } from "@/components/dashboard/footprint-split";
import { KpiStrip } from "@/components/dashboard/kpi-strip";
import { OverviewHeader } from "@/components/dashboard/overview-header";
import { RecentActivity } from "@/components/dashboard/recent-activity";
import { RegionExplorer } from "@/components/dashboard/region-explorer";
import { TopRecommendations } from "@/components/dashboard/right-column";
import { ServicesBoard } from "@/components/dashboard/services-board";
import { SetupGuide } from "@/components/dashboard/setup-guide";
import { StatCardsNotice } from "@/components/dashboard/stat-cards";
import { TrendPanel } from "@/components/dashboard/trend-panel";
import { WorkloadEfficiency } from "@/components/dashboard/workload-efficiency";

import { getUser } from "@/lib/auth/session";
import { canWrite } from "@/lib/permissions";

export const metadata = { title: "Overview · Reliabilix GreenOps" };

export default async function OverviewPage() {
  const user = (await getUser())!;
  return (
    <div className="mx-auto max-w-[1680px] space-y-3">
      <div className="grid grid-cols-[minmax(0,1fr)] gap-3 min-[1500px]:grid-cols-[minmax(0,1fr)_340px]">
        <div className="min-w-0 space-y-3">
          <OverviewHeader />
          <DemoBanner canWrite={canWrite(user.role)} />
          <FirstFindings />
          <KpiStrip />
          <CloudCosts />
          <StatCardsNotice />
          <FinopsStrip />
          <div className="grid grid-cols-[minmax(0,1fr)] gap-3 min-[1100px]:grid-cols-[minmax(0,1.75fr)_minmax(0,1fr)]">
            <TrendPanel />
            <FootprintSplit />
          </div>
          <ServicesBoard />
          <RegionExplorer />
          <div className="grid grid-cols-[minmax(0,1fr)] gap-3 min-[900px]:grid-cols-2">
            <WorkloadEfficiency />
            <DataQuality />
          </div>
        </div>
        <div className="col-divider grid grid-cols-[minmax(0,1fr)] content-start items-start gap-3 min-[760px]:max-[1499px]:grid-cols-2 min-[1500px]:pt-0">
          <SetupGuide />
          <TopRecommendations />
          <ConnectedAccounts />
          <RecentActivity />
        </div>
      </div>
    </div>
  );
}
