"use client";

import { PieChart } from "lucide-react";
import Link from "next/link";
import { Card, CardHeader } from "@/components/ui/card";
import { EmptyState, ErrorState } from "@/components/ui/empty-state";
import { Skeleton } from "@/components/ui/skeleton";
import { ProviderDonut } from "@/components/charts/provider-donut";
import { content } from "@/content/schema";
import { useProviders } from "@/hooks/use-overview";

export function CarbonByProvider() {
  const q = useProviders();
  return (
    <Card index={5} interactive={false}>
      <CardHeader title="Carbon by Cloud Provider" action={<Link href="/cloud" className="text-xs font-semibold text-primary hover:underline">Accounts →</Link>} />
      {q.isLoading ? <Skeleton className="h-[220px]" /> : q.isError ? <ErrorState /> :
        q.data!.length === 0 ? <EmptyState icon={PieChart} text={content.empty.noUsage} className="h-[220px]" /> :
        <div className="flex min-h-[250px] items-center"><div className="w-full"><ProviderDonut shares={q.data!} /></div></div>}
    </Card>
  );
}
