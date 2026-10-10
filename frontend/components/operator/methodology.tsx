"use client";

import { Badge } from "@/components/ui/badge";
import { Card, CardHeader } from "@/components/ui/card";
import { ErrorState } from "@/components/ui/empty-state";
import { Skeleton } from "@/components/ui/skeleton";
import { useOperatorMethodology } from "@/hooks/use-admin";
import { PageTitle, th } from "./parts";

const UNIT: Record<string, string> = { vcpu_hours: "kWh per vCPU-hour", memory_gb_hours: "kWh per GB-hour", storage_gb_months: "kWh per GB-month" };

/** Every coefficient set the platform has: what is active, where the numbers come from and their checksum. Read-only. */
export function MethodologyView() {
  const q = useOperatorMethodology();
  return (
    <div className="space-y-5">
      <PageTitle title="Methodology & SCI" sub="The calculation model: coefficient sets are immutable and versioned; a new set is a new version and old results keep the version they were calculated with. Customers compare versions on their own data on Carbon & Energy." />
      {q.isLoading ? <Skeleton className="h-64" /> : q.isError ? <ErrorState /> : q.data!.map((m, i) => (
        <Card key={m.version} index={i} interactive={false}>
          <CardHeader title={<span className="flex flex-wrap items-center gap-2">{m.version}{m.active && <Badge tone="good">active</Badge>}<Badge tone={m.status === "provisional" ? "warn" : "good"}>{m.status}</Badge></span>} />
          <p className="text-sm text-muted">{m.notes}</p>
          <dl className="mt-4 grid gap-x-8 gap-y-3 text-sm sm:grid-cols-2">
            <div><dt className="text-xs font-medium text-muted">Provenance</dt><dd><code className="text-xs">{m.provenance}</code></dd></div>
            <div><dt className="text-xs font-medium text-muted">Checksum (SHA-256)</dt><dd className="break-all"><code className="text-xs">{m.checksum}</code></dd></div>
            <div><dt className="text-xs font-medium text-muted">PUE</dt><dd>{m.pue}</dd></div>
            <div><dt className="text-xs font-medium text-muted">Embodied emissions (M)</dt><dd>{m.embodied_g_per_kwh > 0 ? `${m.embodied_g_per_kwh} g per kWh` : "not included"}</dd></div>
          </dl>
          <div className="mt-4 grid gap-6 lg:grid-cols-2">
            <div><p className="mb-1 text-xs font-medium text-muted">Measured usage</p>
              <table className="w-full text-sm"><thead><tr><th className={th}>Kind</th><th className={th}>Coefficient</th></tr></thead>
                <tbody className="divide-y divide-line/70">{Object.entries(m.energy_kwh_per_unit).map(([k, v]) => <tr key={k}><td className="py-1.5 pr-3">{k}</td><td>{v} <span className="text-xs text-muted">{UNIT[k] ?? ""}</span></td></tr>)}</tbody></table></div>
            <div><p className="mb-1 text-xs font-medium text-muted">Estimated from cost (low confidence)</p>
              <table className="w-full text-sm"><thead><tr><th className={th}>Category</th><th className={th}>kWh per USD</th></tr></thead>
                <tbody className="divide-y divide-line/70">{Object.entries(m.cost_based_kwh_per_usd).map(([k, v]) => <tr key={k}><td className="py-1.5 pr-3">{k}</td><td>{v}</td></tr>)}</tbody></table></div>
          </div>
          {m.caveats.length > 0 && <ul className="mt-4 list-disc space-y-1 pl-5 text-xs text-muted">{m.caveats.map((c) => <li key={c}>{c}</li>)}</ul>}
        </Card>
      ))}
    </div>
  );
}
