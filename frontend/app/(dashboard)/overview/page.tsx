import { CarbonByProvider } from "@/components/dashboard/carbon-by-provider";
import { ConnectedAccounts } from "@/components/dashboard/connected-accounts";
import { CostVsCarbon } from "@/components/dashboard/cost-vs-carbon";
import { RecentActivity } from "@/components/dashboard/recent-activity";
import { RegionIntensity } from "@/components/dashboard/region-intensity";
import { PromoBanner, Tagline, TopRecommendations } from "@/components/dashboard/right-column";
import { ServicesTable } from "@/components/dashboard/services-table";
import { StatCards } from "@/components/dashboard/stat-cards";
import { content } from "@/content/schema";

export const metadata = { title: "Overview · Reliabilix GreenOps" };

export default function OverviewPage() {
  return (
    <div className="space-y-5">
      <div>
        <h1 className="text-2xl font-bold tracking-tight">{content.pageTitle}</h1>
        <p className="mt-1 text-sm text-muted">{content.pageSubtitle}</p>
      </div>
      <div className="grid gap-5 xl:grid-cols-[minmax(0,1fr)_340px]">
        <div className="space-y-5">
          <StatCards />
          <div className="grid gap-5 lg:grid-cols-[minmax(0,3fr)_minmax(0,2fr)]">
            <CostVsCarbon />
            <CarbonByProvider />
          </div>
          <ServicesTable />
          <div className="grid gap-5 lg:grid-cols-2">
            <RegionIntensity />
            <div className="space-y-5"><RecentActivity /><ConnectedAccounts /></div>
          </div>
        </div>
        <div className="space-y-5">
          <PromoBanner />
          <TopRecommendations />
          <Tagline />
        </div>
      </div>
    </div>
  );
}
