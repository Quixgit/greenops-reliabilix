"use client";

import { PieChart } from "lucide-react";
import { Card, CardHeader } from "@/components/ui/card";
import { EmptyState, ErrorState } from "@/components/ui/empty-state";
import { Skeleton } from "@/components/ui/skeleton";
import { ProviderDonut } from "@/components/charts/provider-donut";
import { content } from "@/content/schema";
import { useProviders } from "@/hooks/use-overview";

export function CarbonByProvider() {
  const q = useProviders();
  return (
    <Card>
      <CardHeader title="Carbon by Cloud Provider" />
      {q.isLoading ? <Skeleton className="h-[220px]" /> : q.isError ? <ErrorState /> :
        q.data!.length === 0 ? <EmptyState icon={PieChart} text={content.empty.noUsage} className="h-[220px]" /> :
        <ProviderDonut shares={q.data!} />}
    </Card>
  );
}
