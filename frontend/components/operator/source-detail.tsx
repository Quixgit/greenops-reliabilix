"use client";

import { ArrowLeft, Cloud } from "lucide-react";
import Link from "next/link";
import { Badge } from "@/components/ui/badge";
import { Card, CardHeader } from "@/components/ui/card";
import { EmptyState, ErrorState } from "@/components/ui/empty-state";
import { Skeleton } from "@/components/ui/skeleton";
import { useOperatorApiUsage, useOperatorDataSources, useOperatorIntegrations, useOperatorMe } from "@/hooks/use-admin";
import { shortTime } from "@/lib/format-time";
import { ACCESS, CATEGORY, PAYMENT_IDS, STATE } from "./data-sources";
import { IntegrationForm } from "./integrations";
import { Chip } from "./parts";

const PROVIDER_OF: Record<string, string> = { "aws-cost": "aws", "gcp-billing": "gcp", "azure-billing": "azure" };

/** Requests made to the provider: today against the daily cap, the last 7 and 30 days, and failures. Counts only. */
function RequestUsage({ provider }: { provider: string }) {
  const q = useOperatorApiUsage();
  if (q.isLoading) return <Skeleton className="h-24" />;
  if (q.isError || !q.data) return null;
  const rows = q.data.items.filter((r) => r.provider === provider);
  const today = new Date().toISOString().slice(0, 10);
  const sum = (days: number, f: (r: (typeof rows)[number]) => number) =>
    rows.filter((r) => Date.now() - new Date(r.day).getTime() < days * 86_400_000).reduce((n, r) => n + f(r), 0);
  const todayN = rows.find((r) => r.day.slice(0, 10) === today)?.requests ?? 0;
  const cap = q.data.daily_caps[provider] ?? 0;
  const pct = cap > 0 ? Math.min(100, (todayN / cap) * 100) : 0;
  return (
    <Card interactive={false}>
      <CardHeader title="Requests to this provider" />
      <p className="text-sm">Today <b className="tabular-nums">{todayN}</b> {cap > 0 ? <>of <b className="tabular-nums">{cap}</b> allowed</> : "(no daily cap)"}</p>
      {cap > 0 && <div className="mt-2 h-2 overflow-hidden rounded-full bg-slate-100"><div className={`h-full rounded-full ${pct >= 100 ? "bg-bad" : pct >= 80 ? "bg-amber-500" : "bg-emerald-500"}`} style={{ width: `${Math.max(2, pct)}%` }} /></div>}
      <dl className="mt-3 grid grid-cols-3 gap-3 text-sm">
        <div><dt className="text-xs text-muted">Last 7 days</dt><dd className="tabular-nums">{sum(7, (r) => r.requests)}</dd></div>
        <div><dt className="text-xs text-muted">Last 30 days</dt><dd className="tabular-nums">{sum(30, (r) => r.requests)}</dd></div>
        <div><dt className="text-xs text-muted">Failed (30 d)</dt><dd className="tabular-nums">{sum(30, (r) => r.errors)}</dd></div>
      </dl>
      <p className="mt-3 text-xs text-muted">At the cap no more requests are made until the next UTC day and stored readings are used. The cap is a setting under “Which source is used”. Cost per request depends on the provider’s plan and is not computed here.</p>
    </Card>
  );
}

/** One page per source or service: what it is and in what state on one side, its settings on the other. */
export function SourceDetailView({ id }: { id: string }) {
  const sources = useOperatorDataSources();
  const ints = useOperatorIntegrations();
  const me = useOperatorMe();
  const canEdit = me.data?.role === "admin";

  if (sources.isLoading || ints.isLoading) return <Skeleton className="h-64" />;
  if (sources.isError || ints.isError) return <ErrorState />;
  const source = sources.data!.find((s) => s.id === id);
  const st = ints.data!.find((i) => i.id === (source ? source.integration_id : id));
  if (!source && !st) return <EmptyState icon={Cloud} text="There is no such source or service." action={{ href: "/operator/data-sources", label: "Back to data sources" }} />;

  const provider = PROVIDER_OF[id];
  const state = source ? STATE[source.state] : null;

  return (
    <div className="space-y-5">
      <div>
        <Link href={PAYMENT_IDS.includes(id) ? "/operator/plans" : `/operator/data-sources${source ? "" : "?tab=services"}`} className="inline-flex items-center gap-1 text-sm text-primary hover:underline"><ArrowLeft className="size-3.5" aria-hidden />{PAYMENT_IDS.includes(id) ? "Plans & billing" : "Data sources"}</Link>
        <div className="mt-1 flex flex-wrap items-center gap-2">
          <h1 className="text-2xl font-bold tracking-tight">{source?.name ?? st!.name}</h1>
          {source && <span className={`inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium ring-1 ring-inset ${ACCESS[source.access].cls}`}>{ACCESS[source.access].label}</span>}
          {state && <Badge tone={state.tone}>{state.label}</Badge>}
          {st?.prepared && <Badge tone="stub">Prepared · not used yet</Badge>}
          {st?.startup && <Badge tone="neutral">Applied at restart</Badge>}
        </div>
        {source && <p className="mt-1 text-sm text-muted">{CATEGORY[source.category] ?? source.category}</p>}
      </div>

      <div className="grid gap-4 xl:grid-cols-5">
        <div className="space-y-4 xl:col-span-3">
          <Card interactive={false}>
            <CardHeader title="Settings" action={st ? <Badge tone={st.ready ? "good" : "warn"}>{st.ready ? (st.prepared ? "Stored" : "Complete") : "Needs setup"}</Badge> : undefined} />
            {st ? (<>
              <p className="-mt-2 mb-4 text-sm text-muted">{st.description} A value set here wins over the deployment environment; secrets are stored sealed and are never shown again.</p>
              <IntegrationForm st={st} canEdit={!!canEdit} />
              {!canEdit && <p className="mt-3 text-xs text-muted">Only an operator with the admin role can change settings.</p>}
            </>) : (
              <p className="text-sm text-muted">This source has no platform-level settings. {source?.access === "customer_provided" ? "Customers connect it themselves in their workspace; the platform only reads it with the access they grant." : source?.access === "internal" ? "It is part of the platform and versioned in code." : ""}</p>
            )}
          </Card>

          {id === "electricitymaps" && <RequestUsage provider="electricitymaps" />}
          {provider && (source?.in_error ?? 0) > 0 && (
            <Link href="/operator/connections" className="flex items-center justify-between gap-3 rounded-2xl bg-red-50 px-4 py-3 text-sm font-medium text-bad ring-1 ring-red-200 hover:bg-red-100">
              {source!.in_error} customer connection{source!.in_error === 1 ? "" : "s"} failing to synchronize<span>View →</span>
            </Link>
          )}
        </div>

        {source && (
          <Card interactive={false} className="xl:col-span-2">
            <CardHeader title="About this source" />
            <dl className="space-y-3 text-sm">
              <div><dt className="text-xs font-medium text-muted">What it provides</dt><dd className="mt-1 flex flex-wrap gap-1.5">{source.metrics.map((m) => <Chip key={m}>{m}</Chip>)}</dd></div>
              <div><dt className="text-xs font-medium text-muted">Coverage</dt><dd className="mt-0.5">{source.coverage}</dd></div>
              <div><dt className="text-xs font-medium text-muted">Terms</dt><dd className="mt-0.5">{source.terms}</dd></div>
              {source.connections != null && (
                <div><dt className="text-xs font-medium text-muted">Customer connections</dt>
                  <dd className="mt-0.5">{source.connections}{source.organizations ? ` in ${source.organizations} organization${source.organizations === 1 ? "" : "s"}` : ""}{source.in_error ? `, ${source.in_error} failing` : ""}{source.last_sync_at ? ` · last sync ${shortTime(source.last_sync_at)}` : ""}</dd></div>
              )}
              {source.note && <div className="rounded-xl bg-amber-50 px-3 py-2 text-xs text-slate-700">{source.note}</div>}
            </dl>
          </Card>
        )}
      </div>
    </div>
  );
}
