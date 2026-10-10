"use client";

import { ScatterChart } from "lucide-react";
import Link from "next/link";
import { Card, CardHeader } from "@/components/ui/card";
import { EmptyState, ErrorState } from "@/components/ui/empty-state";
import { Skeleton } from "@/components/ui/skeleton";
import { ServiceScatter } from "@/components/charts/service-scatter";
import { content } from "@/content/schema";
import { useFinopsSummary, useServices } from "@/hooks/use-overview";
import { useT } from "@/i18n/provider";
import { useUi } from "@/lib/store";

/** The question this card answers: are the most expensive services also the most carbon-intensive? */
export function ServiceFootprint() {
  const t = useT();
  const q = useServices("all");
  useFinopsSummary(); // re-render when the reporting currency is known
  const focus = useUi((s) => s.setFocusService);
  const pick = (name: string) => {
    focus(name);
    document.getElementById("services-table")?.scrollIntoView({ behavior: "smooth", block: "start" });
  };
  return (
    <Card index={5} interactive={false}>
      <CardHeader title={t("ov.svc.title")} action={<Link href="/carbon" className="text-xs font-semibold text-primary hover:underline">{t("ov.svc.link")}</Link>} />
      <p className="-mt-2 mb-2 text-xs text-muted">{t("ov.svc.question")}</p>
      {q.isLoading ? <Skeleton className="h-[300px]" /> : q.isError ? <ErrorState /> :
        q.data!.length === 0 ? <EmptyState icon={ScatterChart} text={content.empty.noUsage} className="h-[300px]" /> :
        <ServiceScatter rows={q.data!} onPick={pick} />}
    </Card>
  );
}
