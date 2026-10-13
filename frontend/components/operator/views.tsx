"use client";



import { IconChevronRight, IconBuilding as Building2, IconGauge as Gauge, IconReceipt as Receipt, IconUsers as Users, IconWarning as AlertTriangle } from "@/components/ui/icons";
import Link from "next/link";
import { useMemo, useState } from "react";

import { SettingsEditor, SettingsHistory } from "@/components/settings/settings-editor";

import { ErrorState } from "@/components/ui/empty-state";

import { Skeleton } from "@/components/ui/skeleton";
import { PillTabs } from "@/components/ui/tabs";
import { useOperatorMe, useOperatorOrgUsage, useOperatorTenants } from "@/hooks/use-admin";


import { type OperatorTenant } from "@/lib/api/schemas";

import { shortTime } from "@/lib/format-time";
import { PageTitle, selectCls, Tile } from "./parts";
import { Avatar, DataTable, EmptyPanel, Pill, RowList, SearchBox, Segmented, type Column } from "./ui";



const planTone = (p: string) => (p === "enterprise" ? "violet" : p === "business" ? "info" : p === "team" ? "good" : "neutral") as "violet" | "info" | "good" | "neutral";
const needs = (t: OperatorTenant) => t.connections_in_error + t.failed_deliveries_7d;
const usd = (n: number) => `$${n.toFixed(2)}`;
type OrgUse = NonNullable<ReturnType<typeof useOperatorOrgUsage>["data"]>["items"][number];

/** Every organization with its plan, what it has, what it costs to serve, and whether it needs a look. */
export function OrganizationsView() {
  const tenants = useOperatorTenants();
  const usage = useOperatorOrgUsage(30);
  const [q, setQ] = useState("");
  const [plan, setPlan] = useState("all");
  const [state, setState] = useState("all");
  const [tab, setTab] = useState("list");
  const cost = useMemo(() => new Map((usage.data?.items ?? []).map((u) => [u.tenant_id, u])), [usage.data]);
  const all = useMemo(() => tenants.data ?? [], [tenants.data]);
  const plans = useMemo(() => Array.from(new Set(all.map((t) => t.plan))).sort(), [all]);
  const rows = useMemo(() => {
    const needle = q.trim().toLowerCase();
    return all.filter((t) => (plan === "all" || t.plan === plan) && (state === "all" || needs(t) > 0) && (!needle || t.name.toLowerCase().includes(needle)));
  }, [all, q, plan, state]);
  const attention = all.filter((t) => needs(t) > 0).length;
  const paid = all.filter((t) => t.plan !== "free").length;
  const totalCost = usage.data?.items.reduce((n, u) => n + u.estimated_cost_usd, 0);

  const num = (k: "usage_rows" | "calculation_rows" | "sync_runs" | "deliveries" | "reports" | "k8s_samples", header: string): Column<OrgUse> =>
    ({ key: k, header, align: "right", hideBelow: "md", sort: (a, b) => a[k] - b[k], render: (u) => <span className="tabular-nums">{u[k].toLocaleString()}</span> });
  const usageCols: Column<OrgUse>[] = [
    { key: "name", header: "Organization", sort: (a, b) => a.name.localeCompare(b.name), render: (u) => <span className="flex items-center gap-3"><Avatar name={u.name} size="sm" />{u.name}</span> },
    { key: "plan", header: "Plan", render: (u) => <Pill tone={planTone(u.plan)}>{u.plan}</Pill> },
    num("usage_rows", "Usage rows"), num("calculation_rows", "Calculations"), num("sync_runs", "Sync runs"), num("deliveries", "Deliveries"), num("reports", "Reports"), num("k8s_samples", "K8s samples"),
    { key: "cost", header: "Estimated cost", align: "right", sort: (a, b) => a.estimated_cost_usd - b.estimated_cost_usd, render: (u) => <span className="font-semibold tabular-nums">{usd(u.estimated_cost_usd)}</span> },
  ];

  return (
    <div className="space-y-6">
      <PageTitle title="Organizations" sub="Counts and statuses only. Open an organization for its plan, limits, module switches, timeline and change history." />
      <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
        <Tile index={0} label="Organizations" value={tenants.isLoading ? undefined : all.length} icon={Building2} color="sage" />
        <Tile index={1} label="On a paid plan" value={tenants.isLoading ? undefined : paid} icon={Receipt} color="lavender" note={all.length ? `${Math.round((paid / all.length) * 100)}% of all` : undefined} />
        <Tile index={2} label="Need a look" value={tenants.isLoading ? undefined : attention} icon={AlertTriangle} color="peach" tone={attention ? "warn" : undefined} note="failing connections or deliveries" />
        <Tile index={3} label="Cost of serving, 30 d (USD)" value={totalCost == null ? undefined : Math.round(totalCost * 100) / 100} icon={Gauge} color="butter" note="an estimate, not an accounting figure" />
      </div>

      <PillTabs value={tab} onChange={setTab} label="Organizations sections" tabs={[{ value: "list", label: "Organizations" }, { value: "usage", label: "Resource use and cost" }]} />

      {tab === "list" ? (
        <div className="space-y-4">
          <div className="flex flex-wrap items-center gap-3">
            <SearchBox value={q} onChange={setQ} placeholder="Search organizations…" className="w-full sm:w-72" />
            <Segmented label="Plan" value={plan} onChange={setPlan} options={[{ value: "all", label: "All plans" }, ...plans.map((p) => ({ value: p, label: p, count: all.filter((t) => t.plan === p).length }))]} />
            <Segmented label="State" value={state} onChange={setState} options={[{ value: "all", label: "All" }, { value: "attention", label: "Needs a look", count: attention }]} />
            <span className="ml-auto text-xs text-muted">{rows.length} of {all.length}</span>
          </div>
          {tenants.isLoading ? <Skeleton className="h-40" /> : tenants.isError ? <ErrorState /> : (
            <RowList rows={[...rows].sort((x, y) => needs(y) - needs(x) || y.created_at.localeCompare(x.created_at))} rowKey={(t) => t.id} pageSize={8}
              empty={all.length === 0 ? <EmptyPanel tone="quiet" icon={Building2} title="No organizations yet" text="An organization appears here the moment someone signs up." /> : <EmptyPanel tone="quiet" icon={Users} title="No organization matches" text="Change the search or the filters." />}
              row={(t) => (
                <Link href={`/operator/organizations/${t.id}`} className="group flex flex-wrap items-center gap-x-4 gap-y-2 rounded-2xl px-3 py-3.5 transition-colors hover:bg-slate-50">
                  <Avatar name={t.name} size="lg" />
                  <span className="min-w-0 flex-1 basis-56">
                    <span className="flex flex-wrap items-center gap-2 font-semibold group-hover:text-primary">{t.name}<Pill tone={planTone(t.plan)}>{t.plan}</Pill></span>
                    <span className="mt-0.5 block text-xs text-muted">since {shortTime(t.created_at)}</span>
                  </span>
                  <span className="flex flex-wrap items-center gap-x-5 gap-y-1 text-sm text-muted">
                    <span><b className="tabular-nums text-ink">{t.members}</b> members</span>
                    <span><b className="tabular-nums text-ink">{t.projects}</b> projects</span>
                    <span><b className="tabular-nums text-ink">{t.connections}</b> connection{t.connections === 1 ? "" : "s"}{t.connections_in_error > 0 && <span className="ml-1 font-medium text-[#b8625c]">· {t.connections_in_error} failing</span>}</span>
                    {t.failed_deliveries_7d > 0 && <Pill tone="warn">{t.failed_deliveries_7d} failed deliveries</Pill>}
                  </span>
                  <span className="w-24 text-right"><span className="block text-sm font-semibold tabular-nums">{cost.has(t.id) ? usd(cost.get(t.id)!.estimated_cost_usd) : "—"}</span><span className="block text-[11px] text-muted">serving, 30 d</span></span>
                  {needs(t) > 0 ? <Pill tone="warn" pulse>needs a look</Pill> : <Pill tone="good">healthy</Pill>}
                  <IconChevronRight className="size-4 text-slate-300 transition-transform group-hover:translate-x-0.5 group-hover:text-slate-500" aria-hidden />
                </Link>
              )} />
          )}
        </div>
      ) : (
        <div className="space-y-3">
          <p className="max-w-3xl text-sm text-muted">Counts of records and runs per organization over the last 30 days, and an estimate of what serving it costs: the counts times the unit costs set under Platform settings → Cost of serving an organization. It is for pricing and limits, not for accounting.</p>
          {usage.isLoading ? <Skeleton className="h-40" /> : usage.isError ? <ErrorState /> : (
            <DataTable rows={usage.data!.items} rowKey={(u) => u.tenant_id} href={(u) => `/operator/organizations/${u.tenant_id}`} columns={usageCols} initialSort={{ key: "cost", dir: "desc" }} empty="No organizations yet." />
          )}
        </div>
      )}
    </div>
  );
}

// Edited on Aggregation & policies, with controls that fit them; not repeated here.
const ON_AGGREGATION = ["carbon.provider_priority", "carbon.region_provider_overrides", "carbon.source_max_age_hours", "carbon.min_day_readings", "carbon.max_reading_age_hours"];

export function PlatformSettingsView() {
  const me = useOperatorMe();
  const [tab, setTab] = useState<"settings" | "history">("settings");
  return (
    <div className="space-y-6">
      <PageTitle title="Platform settings" sub="Limits, module switches, security, retention. Every change is written to the history." />
      <PillTabs value={tab} onChange={(v) => setTab(v as "settings" | "history")} label="Platform settings sections" tabs={[{ value: "settings", label: "Settings" }, { value: "history", label: "Change history" }]} />
      <div key={tab} className="page-enter">
        {tab === "settings"
          ? <SettingsEditor mode="platform" path="operator/settings" historyPath="operator/settings/history" canEdit={me.data?.role === "admin"}
              hideKeys={ON_AGGREGATION} hideGroups={["organization"]}
              groupNotes={{ carbon: <>The order of sources, a source for one region, how many readings a day needs and how old a reading may be are set on <Link href="/operator/aggregation" className="font-semibold text-primary hover:underline">Aggregation &amp; policies</Link>. Suspending an organization is done on its own page under Organizations.</> }} />
          : <SettingsHistory path="operator/settings/history?limit=200" />}
      </div>
    </div>
  );
}
