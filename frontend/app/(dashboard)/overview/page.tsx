import { ConnectedAccounts } from "@/components/dashboard/connected-accounts";
import { DataQuality } from "@/components/dashboard/data-quality";
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

export const metadata = { title: "Overview · Reliabilix GreenOps" };

export default function OverviewPage() {
  return (
    <div className="mx-auto max-w-[1680px] space-y-5">
      <div className="grid grid-cols-[minmax(0,1fr)] gap-5 xl:grid-cols-[minmax(0,1fr)_380px]">
        <div className="min-w-0 space-y-5">
          <OverviewHeader />
          <KpiStrip />
          <StatCardsNotice />
          <FinopsStrip />
          <div className="grid grid-cols-[minmax(0,1fr)] gap-5 lg:grid-cols-[minmax(0,1.7fr)_minmax(0,1fr)]">
            <TrendPanel />
            <FootprintSplit />
          </div>
          <ServicesBoard />
          <RegionExplorer />
          <div className="grid grid-cols-[minmax(0,1fr)] gap-5 lg:grid-cols-[minmax(0,1fr)_minmax(0,1fr)]">
            <WorkloadEfficiency />
            <DataQuality />
          </div>
        </div>
        <div className="col-divider space-y-5 xl:pt-1">
          <SetupGuide />
          <TopRecommendations />
          <ConnectedAccounts />
          <RecentActivity />
        </div>
      </div>
    </div>
  );
}
