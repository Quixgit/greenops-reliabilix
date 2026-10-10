"use client";

import { Info, LineChart } from "lucide-react";
import { Card, CardHeader } from "@/components/ui/card";
import { EmptyState, ErrorState } from "@/components/ui/empty-state";
import { Skeleton } from "@/components/ui/skeleton";
import { CostCarbonChart } from "@/components/charts/cost-carbon-chart";
import { content } from "@/content/schema";
import { useTrend } from "@/hooks/use-overview";

export function CostVsCarbon() {
  const q = useTrend();
  return (
    <Card index={4} interactive={false}>
      <CardHeader title={<>Cost vs Carbon Footprint <span title="Daily cost (left axis) and carbon (right axis) for the selected period"><Info className="size-3.5 text-slate-400" /></span></>} />
      {q.isLoading ? <Skeleton className="h-[300px]" /> : q.isError ? <ErrorState /> :
        q.data!.length === 0 ? <EmptyState icon={LineChart} text={content.empty.noUsage} className="h-[300px]" /> :
        <CostCarbonChart points={q.data!} />}
    </Card>
  );
}
