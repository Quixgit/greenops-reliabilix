"use client";

import { LineChart } from "lucide-react";
import { Card, CardHeader } from "@/components/ui/card";
import { EmptyState, ErrorState } from "@/components/ui/empty-state";
import { Skeleton } from "@/components/ui/skeleton";
import { CostCarbonChart } from "@/components/charts/cost-carbon-chart";
import { content } from "@/content/schema";
import { useTrend } from "@/hooks/use-overview";

export function CostVsCarbon() {
  const q = useTrend();
  return (
    <Card>
      <CardHeader title="Cost vs Carbon Footprint" />
      {q.isLoading ? <Skeleton className="h-[300px]" /> : q.isError ? <ErrorState /> :
        q.data!.length === 0 ? <EmptyState icon={LineChart} text={content.empty.noUsage} className="h-[300px]" /> :
        <CostCarbonChart points={q.data!} />}
    </Card>
  );
}
