"use client";

import { IconArrowLeft as ArrowLeft, IconCloud as Cloud, IconDatabase, IconGauge, IconInfo, IconKey, IconPlug, IconRefresh, IconWarning } from "@/components/ui/icons";
import Link from "next/link";
import { Card, CardHeader } from "@/components/ui/card";
import { EmptyState, ErrorState } from "@/components/ui/empty-state";
import { Skeleton } from "@/components/ui/skeleton";
import { useOperatorApiUsage, useOperatorDataSources, useOperatorIntegrations, useOperatorMe } from "@/hooks/use-admin";
import { useAggregation } from "@/hooks/use-operator-extra";
import { shortTime } from "@/lib/format-time";
import { ACCESS, CATEGORY, PAYMENT_IDS, STATE } from "./data-sources";
import { ProviderMark } from "@/components/connectors/provider-mark";
import { IntegrationForm } from "./integrations";
import { Chip, TextTile, Tile } from "./parts";
import { Pill } from "./ui";

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
      {cap > 0 && <div className="mt-2 h-2 overflow-hidden rounded-full bg-slate-100"><div className={`h-full rounded-full ${pct >= 100 ? "bg-pastel-rose" : pct >= 80 ? "bg-pastel-amber" : "bg-leaf"}`} style={{ width: `${Math.max(2, pct)}%` }} /></div>}
      <dl className="mt-3 grid grid-cols-3 gap-3 text-sm">
        <div><dt className="text-xs text-muted">Last 7 days</dt><dd className="tabular-nums">{sum(7, (r) => r.requests)}</dd></div>
        <div><dt className="text-xs text-muted">Last 30 days</dt><dd className="tabular-nums">{sum(30, (r) => r.requests)}</dd></div>
        <div><dt className="text-xs text-muted">Failed (30 d)</dt><dd className="tabular-nums">{sum(30, (r) => r.errors)}</dd></div>
      </dl>
      <p className="mt-3 text-xs text-muted">At the cap no more requests are made until the next UTC day and stored readings are used. The cap is a setting under “Which source is used”. Cost per request depends on the provider’s plan and is not computed here.</p>
    </Card>
  );
}

const AGO = (iso: string | null) => {
  if (!iso) return "none yet";
  const h = (Date.now() - new Date(iso).getTime()) / 3_600_000;
  return h < 1 ? "under an hour ago" : h < 48 ? `${Math.round(h)} h ago` : `${Math.round(h / 24)} days ago`;
};

/** What matters about a grid source: where it stands in the order, how much it covers and how fresh it is. Customers do not connect these. */
function GridFacts({ id, missing, ready }: { id: string; missing: number; ready?: boolean }) {
  const agg = useAggregation();
  const s = agg.data?.sources.find((x) => x.id === id);
  const used = (s?.position ?? 0) > 0;
  return (
    <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
      <TextTile index={0} label="Place in the order" text={agg.data ? (used ? `${s!.position} of ${agg.data.policy.priority.length}` : "not used") : "…"} icon={IconDatabase} color="lavender"
        note={used ? "the first source with enough readings wins" : "never used until it is added to the order"} />
      <TextTile index={1} label="Regions with data" text={s ? String(s.regions) : "…"} icon={IconGauge} color="sky" note={s ? `${s.hourly_readings_30d.toLocaleString()} hourly readings in 30 days` : undefined} />
      <TextTile index={2} label="Newest reading" text={s ? AGO(s.newest) : "…"} icon={IconRefresh} color={s?.freshness === "stale" || s?.freshness === "none" ? "butter" : "sage"} note={s ? `accepted age ${s.max_age_hours} h` : undefined} />
      <TextTile index={3} label="Key" text={missing ? "missing" : ready ? "in place" : "—"} icon={IconKey} color={missing ? "butter" : "sage"} note={missing ? "this source is skipped until it is set" : "sealed, never shown"} />
    </div>
  );
}

const ACCESS_TONE = { free: "good", paid: "warn", customer_provided: "info", internal: "violet" } as const;
const STATE_TONE = { good: "good", warn: "warn", bad: "bad", neutral: "neutral", stub: "violet" } as const;

function Fact({ label, children }: { label: string; children: React.ReactNode }) {
  return <div><dt className="text-xs font-medium uppercase tracking-wide text-slate-400">{label}</dt><dd className="mt-1 text-sm">{children}</dd></div>;
}

/** One page per source or service: what it is and in what state on top, its settings on one side, what it is on the other. */
export function SourceDetailView({ id }: { id: string }) {
  const sources = useOperatorDataSources();
  const ints = useOperatorIntegrations();
  const me = useOperatorMe();
  const canEdit = me.data?.role === "admin";

  if (sources.isLoading || ints.isLoading) return <div className="space-y-4"><Skeleton className="h-28" /><Skeleton className="h-64" /></div>;
  if (sources.isError || ints.isError) return <ErrorState />;
  const source = sources.data!.find((s) => s.id === id);
  const st = ints.data!.find((i) => i.id === (source ? source.integration_id : id));
  if (!source && !st) return <EmptyState icon={Cloud} text="There is no such source or service." action={{ href: "/operator/data-sources", label: "Back to data sources" }} />;

  const provider = PROVIDER_OF[id];
  const state = source ? STATE[source.state] : null;
  const markId = provider ?? id;
  const back = PAYMENT_IDS.includes(id) ? { href: "/operator/plans", label: "Plans & billing" } : { href: `/operator/data-sources${source ? "" : "?tab=services"}`, label: "Data sources" };
  const missing = st?.fields.filter((f) => f.required && f.source === "not_set").length ?? 0;

  return (
    <div className="space-y-6">
      <Link href={back.href} className="inline-flex items-center gap-1.5 rounded-lg px-2 py-1 text-sm font-medium text-slate-500 transition hover:bg-slate-100 hover:text-ink">
        <ArrowLeft className="size-3.5" aria-hidden />{back.label}
      </Link>

      <div className="glass rise flex flex-wrap items-center gap-5 p-5">
        <span className="grid size-16 shrink-0 place-items-center rounded-2xl bg-slate-50 ring-1 ring-line/70"><ProviderMark id={markId} size="lg" /></span>
        <div className="min-w-0 flex-1">
          <h1 className="text-2xl font-bold tracking-tight">{source?.name ?? st!.name}</h1>
          {source && <p className="mt-0.5 text-sm text-muted">{CATEGORY[source.category] ?? source.category}</p>}
          <div className="mt-3 flex flex-wrap gap-2">
            {source && <Pill tone={ACCESS_TONE[source.access]}>{ACCESS[source.access].label}</Pill>}
            {state && <Pill tone={STATE_TONE[state.tone]} pulse={source?.state === "attention"}>{state.label}</Pill>}
            {st?.prepared && <Pill tone="violet">Prepared · not used yet</Pill>}
            {st?.startup && <Pill tone="neutral">Applied at restart</Pill>}
          </div>
        </div>
      </div>

      {source && source.category === "grid_intensity" && <GridFacts id={source.integration_id || source.id} missing={missing} ready={st?.ready} />}
      {source && source.category !== "grid_intensity" && (
        <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
          <Tile index={0} label="Customer connections" value={source.connections} icon={IconPlug} color="sky" note={source.organizations ? `in ${source.organizations} organization${source.organizations === 1 ? "" : "s"}` : undefined} />
          <Tile index={1} label="Failing" value={source.in_error ?? 0} icon={IconWarning} color="peach" tone={source.in_error ? "bad" : undefined} />
          <Tile index={2} label="Missing settings" value={st ? missing : null} icon={IconKey} color={missing ? "butter" : "sage"} tone={missing ? "warn" : undefined} note={st ? (st.ready ? "settings complete" : "needs setup") : "no platform settings"} />
          <div className="glass rise p-4" style={{ "--i": 3 } as React.CSSProperties}>
            <p className="text-xs font-medium text-muted">Last sync</p>
            <p className="mt-2 text-lg font-semibold tracking-tight">{source.last_sync_at ? shortTime(source.last_sync_at) : "—"}</p>
          </div>
        </div>
      )}

      <div className="grid gap-5 xl:grid-cols-5">
        <div className="space-y-5 xl:col-span-3">
          <Card index={1} interactive={false}>
            <CardHeader title="Settings" icon={<IconKey className="size-4 text-leaf-deep" />} action={st ? <Pill tone={st.ready ? "good" : "warn"}>{st.ready ? (st.prepared ? "Stored" : "Complete") : "Needs setup"}</Pill> : undefined} />
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
            <Link href="/operator/connections" className="flex items-center justify-between gap-3 rounded-2xl bg-rose-50 px-4 py-3 text-sm font-medium text-[#c2566a] ring-1 ring-rose-200 transition hover:bg-rose-100">
              {source!.in_error} customer connection{source!.in_error === 1 ? "" : "s"} failing to synchronize<span>View →</span>
            </Link>
          )}
        </div>

        {source && (
          <Card index={2} interactive={false} className="h-fit xl:col-span-2">
            <CardHeader title="About this source" icon={<IconInfo className="size-4 text-leaf-deep" />} />
            <dl className="space-y-4">
              <Fact label="What it provides"><span className="flex flex-wrap gap-1.5">{source.metrics.map((m) => <Chip key={m}>{m}</Chip>)}</span></Fact>
              <Fact label="Coverage">{source.coverage}</Fact>
              <Fact label="Terms">{source.terms}</Fact>
              {source.note && <div className="rounded-xl bg-amber-50 px-3.5 py-2.5 text-sm text-slate-700 ring-1 ring-amber-100">{source.note}</div>}
            </dl>
          </Card>
        )}
      </div>
    </div>
  );
}
