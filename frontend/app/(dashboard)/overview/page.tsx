import { CarbonByProvider } from "@/components/dashboard/carbon-by-provider";
import { ConnectedAccounts } from "@/components/dashboard/connected-accounts";
import { DataQuality } from "@/components/dashboard/data-quality";
import { CostVsCarbon } from "@/components/dashboard/cost-vs-carbon";
import { OverviewHeader } from "@/components/dashboard/overview-header";
import { ServiceFootprint } from "@/components/dashboard/service-footprint";
import { WorkloadEfficiency } from "@/components/dashboard/workload-efficiency";
import { RecentActivity } from "@/components/dashboard/recent-activity";
import { RegionIntensity } from "@/components/dashboard/region-intensity";
import { PromoBanner, Tagline, TopRecommendations } from "@/components/dashboard/right-column";
import { ServicesTable } from "@/components/dashboard/services-table";
import { StatCards } from "@/components/dashboard/stat-cards";
import { content } from "@/content/schema";

export const metadata = { title: "Overview · Reliabilix GreenOps" };

export default function OverviewPage() {
  return (
    <div className="mx-auto max-w-[1680px] space-y-5">
      <div className="grid grid-cols-[minmax(0,1fr)] gap-5 xl:grid-cols-[minmax(0,1fr)_400px]">
        <div className="min-w-0 space-y-5">
          <OverviewHeader />
          <StatCards />
          <div className="grid grid-cols-[minmax(0,1fr)] gap-5 lg:grid-cols-[minmax(0,1.8fr)_minmax(0,1fr)]">
            <CostVsCarbon />
            <CarbonByProvider />
          </div>
          <div className="grid grid-cols-[minmax(0,1fr)] gap-5 lg:grid-cols-[minmax(0,1.8fr)_minmax(0,1fr)]">
            <ServiceFootprint />
            <WorkloadEfficiency />
          </div>
          <RegionIntensity />
          <ServicesTable />
          <div className="grid grid-cols-[minmax(0,1fr)] gap-5 lg:grid-cols-[minmax(0,1.8fr)_minmax(0,1fr)]">
            <RecentActivity />
            <div className="min-w-0 space-y-5">
              <DataQuality />
              <ConnectedAccounts />
            </div>
          </div>
        </div>
        <div className="col-divider space-y-5 xl:pt-1">
          <PromoBanner />
          <TopRecommendations />
        </div>
      </div>
      <Tagline />
    </div>
  );
}
