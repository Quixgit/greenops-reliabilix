"use client";

import { Boxes, Database, HardDrive, Network, Server, Table2, type LucideIcon } from "lucide-react";
import { useState } from "react";
import { Card, CardHeader } from "@/components/ui/card";
import { EmptyState, ErrorState } from "@/components/ui/empty-state";
import { Skeleton } from "@/components/ui/skeleton";
import { PillTabs } from "@/components/ui/tabs";
import { Sparkline } from "@/components/charts/sparkline";
import { content } from "@/content/schema";
import { useServices } from "@/hooks/use-overview";
import { formatCarbon, formatCost } from "@/lib/formatters";

const TABS = [
  { value: "all", label: "All" }, { value: "compute", label: "Compute" }, { value: "storage", label: "Storage" },
  { value: "database", label: "Database" }, { value: "networking", label: "Networking" }, { value: "other", label: "Other" },
];
const ICON: Record<string, LucideIcon> = { compute: Server, storage: HardDrive, database: Database, networking: Network, other: Boxes };

export function ServicesTable() {
  const [tab, setTab] = useState("all");
  const q = useServices(tab);
  return (
    <Card>
      <CardHeader title="Details by Service" action={<PillTabs value={tab} onChange={setTab} tabs={TABS} />} />
      {q.isLoading ? <Skeleton className="h-48" /> : q.isError ? <ErrorState /> :
        q.data!.length === 0 ? <EmptyState icon={Table2} text={content.empty.noUsage} /> : (
          <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <thead>
                <tr className="text-left text-xs uppercase tracking-wide text-muted">
                  <th className="pb-2 font-medium">Service</th><th className="pb-2 text-right font-medium">Cost</th>
                  <th className="pb-2 text-right font-medium">CO₂e</th><th className="pb-2 pl-6 font-medium">Trend</th>
                </tr>
              </thead>
              <tbody>
                {q.data!.map((r) => {
                  const Icon = ICON[r.category] ?? Boxes;
                  return (
                    <tr key={r.service} className="border-t border-line/70">
                      <td className="py-2.5"><span className="flex items-center gap-2.5"><span className="flex size-8 items-center justify-center rounded-lg bg-slate-100 text-slate-500"><Icon className="size-4" /></span>{r.service}</span></td>
                      <td className="py-2.5 text-right tabular-nums">{formatCost(r.cost)}</td>
                      <td className="py-2.5 text-right tabular-nums">{formatCarbon(r.carbon_kg_co2e)}</td>
                      <td className="py-2.5 pl-6"><Sparkline values={r.trend} /></td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>
        )}
    </Card>
  );
}
