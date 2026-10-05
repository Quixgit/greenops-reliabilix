"use client";

import { ArrowDown, ArrowUp, Globe2 } from "lucide-react";
import { Card, CardHeader } from "@/components/ui/card";
import { EmptyState, ErrorState } from "@/components/ui/empty-state";
import { Skeleton } from "@/components/ui/skeleton";
import { RegionMap } from "@/components/charts/region-map";
import { useRegions } from "@/hooks/use-overview";
import { formatPct, nf } from "@/lib/formatters";
import { REGION_LABEL } from "@/lib/regions";
import { cn } from "@/lib/utils";

export function RegionIntensity() {
  const q = useRegions();
  return (
    <Card>
      <CardHeader title="Carbon Intensity by Region" />
      {q.isLoading ? <Skeleton className="h-64" /> : q.isError ? <ErrorState /> :
        q.data!.length === 0 ? <EmptyState icon={Globe2} text="Grid intensity data is collected hourly. It will appear after the first refresh." /> : (
          <>
            <RegionMap regions={q.data!} />
            <table className="mt-3 w-full text-sm">
              <thead><tr className="text-left text-xs uppercase tracking-wide text-muted"><th className="pb-1.5 font-medium">Region</th><th className="pb-1.5 text-right font-medium">gCO₂e/kWh</th><th className="pb-1.5 text-right font-medium">24h</th></tr></thead>
              <tbody>
                {q.data!.map((r) => (
                  <tr key={r.region} className="border-t border-line/70">
                    <td className="py-2">{REGION_LABEL[r.region] ?? r.region}</td>
                    <td className="py-2 text-right tabular-nums">{nf(0).format(r.g_per_kwh)}</td>
                    <td className="py-2 text-right">
                      {r.change_pct === null ? <span className="text-muted">—</span> : (
                        <span className={cn("inline-flex items-center gap-0.5 text-xs font-semibold", r.change_pct <= 0 ? "text-carbon" : "text-bad")}>
                          {r.change_pct <= 0 ? <ArrowDown className="size-3" /> : <ArrowUp className="size-3" />}{formatPct(r.change_pct)}
                        </span>
                      )}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </>
        )}
    </Card>
  );
}
