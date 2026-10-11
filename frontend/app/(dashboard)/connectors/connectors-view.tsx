"use client";

import { useMutation, useQueryClient } from "@tanstack/react-query";
import { ArrowUpRight, CheckCircle2, ChevronDown, Loader2, Search } from "lucide-react";
import { IconBell as Bell, IconBolt as Zap, IconCloud as Cloud, IconPlus as Plus, IconRefresh as RefreshCw, IconWarning as AlertTriangle } from "@/components/ui/icons";
import Link from "next/link";
import { useEffect, useMemo, useState, type CSSProperties } from "react";
import { z } from "zod";
import { ConnectionForm } from "@/components/forms/connection-form";
import { Drawer } from "@/components/connectors/drawer";
import { HealthRing } from "@/components/connectors/health-ring";
import { ProviderMark } from "@/components/connectors/provider-mark";
import { RunBars } from "@/components/connectors/run-bars";
import { Badge } from "@/components/ui/badge";
import { Info } from "@/components/ui/info";
import { Card, CardHeader } from "@/components/ui/card";
import { CountUp } from "@/components/ui/count-up";
import { EmptyState, ErrorState } from "@/components/ui/empty-state";
import { Skeleton } from "@/components/ui/skeleton";
import { useNotificationsEnabled } from "@/hooks/use-admin";
import { useConnectorData } from "@/hooks/use-connectors";
import { useT } from "@/i18n/provider";
import { api } from "@/lib/api/client";
import { connectionState, stateTone, type ConnectionState } from "@/lib/connection-state";
import { connectorSummary, formatDuration, kpis, type ConnectorFacts } from "@/lib/connectors";
import { formatCarbon, formatCost, timeAgo } from "@/lib/formatters";
import { cn } from "@/lib/utils";
import { CATALOG, CATEGORIES, type CatalogItem } from "./catalog";

type Provider = "aws" | "gcp" | "azure";
type Filter = "all" | "synced" | "attention" | "waiting";
const PROVIDER_NAME: Record<string, string> = { aws: "AWS", gcp: "Google Cloud", azure: "Azure" };

const STATE_DOT: Record<ConnectionState, string> = { synced: "bg-emerald-500", waiting: "bg-amber-500", failing: "bg-red-500", attention: "bg-red-500" };

function StateBadge({ state, label }: { state: ConnectionState; label: string }) {
  const live = state === "synced" || state === "waiting";
  return (
    <Badge tone={stateTone(state)} className="gap-1.5 whitespace-nowrap">
      <span className="relative flex size-1.5">
        {live && <span className={cn("absolute inline-flex size-full animate-ping rounded-full opacity-60", STATE_DOT[state])} />}
        <span className={cn("relative inline-flex size-1.5 rounded-full", STATE_DOT[state])} />
      </span>
      {label}
    </Badge>
  );
}

function Kpi({ index, label, hint, info, children, ring }: { index: number; label: string; hint?: string; info?: string; children: React.ReactNode; ring?: React.ReactNode }) {
  return (
    <Card index={index} className="flex items-center justify-between gap-3">
      <div className="min-w-0">
        <p className="flex items-center gap-1 text-sm text-muted">{label}{info && <Info>{info}</Info>}</p>
        <div className="mt-1 text-2xl font-bold tracking-tight">{children}</div>
        {hint && <p className="mt-0.5 truncate text-xs text-muted">{hint}</p>}
      </div>
      {ring}
    </Card>
  );
}

/** Connections of every cloud and the other sources of data in one place: how each is doing, and how to add one. */
export function ConnectorsView({ canConnect }: { canConnect: boolean }) {
  const t = useT();
  const qc = useQueryClient();
  const d = useConnectorData();
  const [filter, setFilter] = useState<Filter>("all");
  const [query, setQuery] = useState("");
  const [cat, setCat] = useState<(typeof CATEGORIES)[number]["id"]>("all");
  const [connectAs, setConnectAs] = useState<Provider | null>(null);
  const [open, setOpen] = useState<string | null>(null);
  // /connectors?connect=aws opens the connect panel for that cloud (the Settings page links here)
  useEffect(() => {
    const p = new URLSearchParams(window.location.search).get("connect");
    if (p === "aws" || p === "gcp" || p === "azure") setConnectAs(p);
  }, []);

  const notifOn = useNotificationsEnabled();
  const conns = d.connections.data ?? [];
  const facts: ConnectorFacts = useMemo(() => ({
    connections: conns, clusters: d.clusters.data ?? [], destinations: d.destinations.data ?? [],
    gridReadings: (d.grid.data ?? []).map((r) => r.at), notificationsEnabled: notifOn,
  }), [conns, d.clusters.data, d.destinations.data, d.grid.data, notifOn]);
  const k = useMemo(() => kpis(facts, d.runs.byId), [facts, d.runs.byId]);
  const projectName = (id: string) => d.projects.data?.find((p) => p.id === id)?.name ?? "—";

  // Cost is the connection's own (its provider and account). Carbon is stored per provider and region, so it is
  // shown only where one connection is the whole provider; with several, a split would be invented.
  const costOf = (id: string) => d.costs.data?.find((x) => x.connection_id === id);
  const carbonOf = (provider: string) => {
    if (conns.filter((c) => c.provider === provider).length !== 1) return null;
    const rows = (d.footprint.data ?? []).filter((r) => r.provider === provider);
    return rows.length ? rows.reduce((a, r) => a + r.carbon_kg_co2e, 0) : null;
  };

  const stateOf = connectionState;
  const counts: Record<Filter, number> = {
    all: conns.length,
    synced: conns.filter((c) => stateOf(c) === "synced").length,
    attention: conns.filter((c) => ["attention", "failing"].includes(stateOf(c))).length,
    waiting: conns.filter((c) => stateOf(c) === "waiting").length,
  };
  const rows = conns.filter((c) => {
    const st = stateOf(c);
    if (filter === "synced" && st !== "synced") return false;
    if (filter === "attention" && st !== "attention" && st !== "failing") return false;
    if (filter === "waiting" && st !== "waiting") return false;
    const q = query.trim().toLowerCase();
    return !q || `${c.provider} ${PROVIDER_NAME[c.provider] ?? ""} ${c.account_ref} ${projectName(c.project_id)}`.toLowerCase().includes(q);
  });
  const broken = conns.filter((c) => ["attention", "failing"].includes(stateOf(c)));

  const sync = useMutation({
    mutationFn: (id: string) => api(`cloud-accounts/${id}/sync`, z.unknown(), undefined, { method: "POST", body: "{}" }),
    onSettled: (_r, _e, id) => {
      void qc.invalidateQueries({ queryKey: ["connections"] });
      void qc.invalidateQueries({ queryKey: ["sync-runs", id] });
    },
  });

  const items = CATALOG.filter((i) => cat === "all" || i.category === cat);
  const loading = d.connections.isLoading;

  return (
    <div className="page-enter space-y-6">
      <div className="flex flex-wrap items-end justify-between gap-4">
        <div>
          <h1 className="text-2xl font-bold tracking-tight">{t("connectors.title")}</h1>
          <p className="mt-1 text-sm text-muted">{t("connectors.subtitle")}</p>
        </div>
        {canConnect && (
          <button onClick={() => setConnectAs((["aws", "gcp", "azure"] as Provider[]).find((p) => !conns.some((c) => c.provider === p)) ?? "aws")} className="brand-gradient inline-flex items-center gap-2 rounded-xl px-4 py-2.5 text-sm font-semibold text-white shadow-sm transition-transform duration-200 hover:-translate-y-0.5 active:translate-y-0">
            <Plus className="size-4" />{t("connectors.connect")}
          </button>
        )}
      </div>

      <div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-4">
        <Kpi index={0} label={t("connectors.kpi.active")} info={t("info.connectors_active")} hint={t("connectors.kpi.active_of", { total: k.total })}
          ring={<span className="flex size-11 items-center justify-center rounded-xl bg-emerald-50 text-emerald-600"><Zap className="size-5" /></span>}>
          {loading ? <Skeleton className="h-8 w-14" /> : <CountUp value={k.active} format={(n) => String(Math.round(n))} />}
        </Kpi>
        <Kpi index={1} label={t("connectors.kpi.sources")} info={t("info.connectors_sources")} hint={k.sources.length ? undefined : t("connectors.kpi.sources_none")}
          ring={<span className="flex size-11 items-center justify-center rounded-xl bg-blue-50 text-blue-600"><Cloud className="size-5" /></span>}>
          {loading ? <Skeleton className="h-8 w-28" /> : k.sources.length ? (
            <span className="flex items-center -space-x-1.5">{k.sources.map((s) => <span key={s} className="rounded-xl ring-2 ring-white"><ProviderMark id={s} size="sm" /></span>)}</span>
          ) : <span className="text-muted">—</span>}
        </Kpi>
        <Kpi index={2} label={t("connectors.kpi.health")} info={t("info.connectors_health")} hint={d.runs.loading ? undefined : k.health === null ? t("connectors.kpi.no_data") : t("connectors.kpi.health_hint")}
          ring={<HealthRing value={d.runs.loading ? null : k.health} />}>
          {d.runs.loading ? <Skeleton className="h-8 w-16" /> : k.health === null ? <span className="text-muted">—</span> : <CountUp value={k.health} format={(n) => `${Math.round(n)}%`} />}
        </Kpi>
        <Kpi index={3} label={t("connectors.kpi.sync_time")} info={t("info.connectors_time")} hint={d.runs.loading ? undefined : k.avgSyncSeconds === null ? t("connectors.kpi.no_data") : t("connectors.kpi.sync_time_hint")}
          ring={<span className="flex size-11 items-center justify-center rounded-xl bg-violet-50 text-violet-600"><RefreshCw className="size-5" /></span>}>
          {d.runs.loading ? <Skeleton className="h-8 w-16" /> : k.avgSyncSeconds === null ? <span className="text-muted">—</span> : formatDuration(k.avgSyncSeconds)}
        </Kpi>
      </div>

      <div className="grid gap-6 xl:grid-cols-[minmax(0,1fr)_340px]">
        <Card index={4} interactive={false} className="min-w-0 !p-0">
          <div className="flex flex-wrap items-center justify-between gap-3 border-b border-line px-5 py-4">
            <h2 className="text-[15px] font-semibold tracking-tight">{t("connectors.section.connections")}</h2>
            <label className="relative">
              <Search className="pointer-events-none absolute left-3 top-1/2 size-4 -translate-y-1/2 text-muted" />
              <input value={query} onChange={(e) => setQuery(e.target.value)} placeholder={t("connectors.search")}
                className="w-56 rounded-xl border border-line bg-white py-2 pl-9 pr-3 text-sm outline-none transition-shadow focus:ring-2 focus:ring-primary/30" />
            </label>
          </div>
          <div role="tablist" className="flex flex-wrap gap-2 border-b border-line px-5 py-3">
            {(["all", "synced", "attention", "waiting"] as Filter[]).map((f) => (
              <button key={f} role="tab" aria-selected={filter === f} onClick={() => setFilter(f)}
                className={cn("inline-flex items-center gap-1.5 rounded-full px-3 py-1 text-sm ring-1 ring-inset transition-all duration-200",
                  filter === f ? "bg-primary text-white ring-primary shadow-sm" : "bg-white text-slate-700 ring-line hover:bg-slate-50")}>
                {t(`connectors.filter.${f}`)}
                <span className={cn("rounded-full px-1.5 text-[11px] tabular-nums", filter === f ? "bg-white/25" : "bg-slate-100 text-muted")}>{counts[f]}</span>
              </button>
            ))}
          </div>

          {loading ? <div className="space-y-3 p-5"><Skeleton className="h-14" /><Skeleton className="h-14" /></div>
            : d.connections.isError ? <ErrorState />
            : conns.length === 0 ? (
              <EmptyState icon={Cloud} text={t("connectors.empty")} className="py-12" />
            ) : rows.length === 0 ? (
              <p className="px-5 py-10 text-center text-sm text-muted">{t("connectors.no_match")}</p>
            ) : (
              <div className="overflow-x-auto">
                <table className="w-full min-w-[720px] text-sm">
                  <thead>
                    <tr className="text-left text-xs uppercase tracking-wide text-muted">
                      <th className="px-5 py-2.5 font-medium">{t("connectors.col.source")}</th>
                      <th className="px-3 py-2.5 font-medium">{t("connectors.col.status")}</th>
                      <th className="px-3 py-2.5 font-medium">{t("connectors.col.last_sync")}</th>
                      <th className="px-3 py-2.5 font-medium">{t("connectors.col.runs")}</th>
                      <th className="px-3 py-2.5 text-right font-medium">{t("connectors.col.cost")}</th>
                      <th className="w-10 px-3 py-2.5" />
                    </tr>
                  </thead>
                  <tbody>
                    {rows.map((c, i) => {
                      const st = stateOf(c);
                      const runs = d.runs.byId[c.id] ?? [];
                      const isOpen = open === c.id;
                      const money = costOf(c.id);
                      const carbon = carbonOf(c.provider);
                      return (
                        <ConnectionRow key={c.id} index={i} open={isOpen} onToggle={() => setOpen(isOpen ? null : c.id)}
                          main={
                            <>
                              <td className="px-5 py-3">
                                <div className="flex items-center gap-3">
                                  <ProviderMark id={c.provider} />
                                  <div className="min-w-0">
                                    <p className="truncate font-medium">{PROVIDER_NAME[c.provider] ?? c.provider}</p>
                                    <p className="truncate text-xs text-muted" title={c.account_ref}>{c.account_ref}</p>
                                  </div>
                                </div>
                              </td>
                              <td className="px-3 py-3"><StateBadge state={st} label={t(`connection.state.${st}`)} /></td>
                              <td className="whitespace-nowrap px-3 py-3 text-muted">{c.last_sync_at ? timeAgo(c.last_sync_at) : t("connectors.never")}</td>
                              <td className="px-3 py-3">{d.runs.loading ? <Skeleton className="h-5 w-20" /> : <RunBars runs={runs} />}</td>
                              <td className="whitespace-nowrap px-3 py-3 text-right">
                                {money ? (
                                  <>
                                    <p className="font-medium tabular-nums">{formatCost(money.cost)}</p>
                                    <p className="text-xs text-muted tabular-nums">{carbon !== null ? `${formatCarbon(carbon)} · ` : ""}{t("connectors.days_with_data", { count: money.days })}</p>
                                  </>
                                ) : <span className="text-muted">—</span>}
                              </td>
                            </>
                          }
                          detail={
                            <div className="grid gap-4 px-5 pb-4 pt-1 sm:grid-cols-[1fr_auto]">
                              <dl className="grid grid-cols-[auto_1fr] gap-x-4 gap-y-1.5 text-sm">
                                <dt className="text-muted">{t("connectors.col.project")}</dt><dd>{projectName(c.project_id)}</dd>
                                <dt className="text-muted">{t("connectors.col.access")}</dt><dd>{t(`connectors.access.${c.access || "role"}`)}</dd>
                                {runs[0]?.quality && <><dt className="text-muted">{t("connectors.col.quality")}</dt><dd>{t(`connectors.quality.${runs[0].quality.status}`)}{runs[0].quality.checked_days > 0 ? ` · ${t("connectors.quality.days", { count: runs[0].quality.checked_days })}` : ""}</dd></>}
                                {runs[0] && <><dt className="text-muted">{t("connectors.last_run")}</dt><dd>{t(`connectors.run.${runs[0].status}`, undefined, runs[0].status)}{runs[0].records ? ` · ${t("connectors.records", { count: runs[0].records })}` : ""}</dd></>}
                                {c.last_error && <><dt className="text-muted">{t("connectors.last_error")}</dt><dd className="text-bad">{c.last_error}</dd></>}
                              </dl>
                              <div className="flex items-start gap-2 sm:flex-col">
                                {canConnect && (
                                  <button onClick={() => sync.mutate(c.id)} disabled={sync.isPending && sync.variables === c.id}
                                    className="inline-flex items-center gap-2 rounded-lg bg-primary px-3 py-1.5 text-sm font-semibold text-white transition-opacity disabled:opacity-60">
                                    {sync.isPending && sync.variables === c.id ? <Loader2 className="size-4 animate-spin" /> : <RefreshCw className="size-4" />}
                                    {sync.isPending && sync.variables === c.id ? t("connectors.syncing") : t("connectors.sync_now")}
                                  </button>
                                )}
                                <Link href={`/cloud/${c.id}`} className="inline-flex items-center gap-1.5 rounded-lg px-3 py-1.5 text-sm font-medium text-primary ring-1 ring-line hover:bg-slate-50">
                                  {t("connectors.open")}<ArrowUpRight className="size-4" />
                                </Link>
                              </div>
                            </div>
                          }
                        />
                      );
                    })}
                  </tbody>
                </table>
              </div>
            )}
        </Card>

        <div className="space-y-6">
          <Card index={5} interactive={false}>
            <CardHeader title={t("connectors.attention.title")} icon={<AlertTriangle className="size-4 text-amber-500" />} />
            {broken.length === 0 ? (
              <p className="flex items-center gap-2 text-sm text-muted"><CheckCircle2 className="size-4 text-emerald-500" />{conns.length ? t("connectors.attention.none") : t("connectors.attention.nothing")}</p>
            ) : (
              <ul className="space-y-3">
                {broken.map((c) => (
                  <li key={c.id} className="rounded-xl bg-red-50/60 p-3 ring-1 ring-red-100">
                    <p className="flex items-center gap-2 text-sm font-medium"><ProviderMark id={c.provider} size="sm" />{PROVIDER_NAME[c.provider] ?? c.provider}</p>
                    <p className="mt-1.5 text-xs text-red-700">{c.last_error ?? t(`connection.state.${stateOf(c)}`)}</p>
                    <Link href={`/cloud/${c.id}`} className="mt-2 inline-flex items-center gap-1 text-xs font-semibold text-primary hover:underline">{t("connectors.attention.fix")}<ArrowUpRight className="size-3.5" /></Link>
                  </li>
                ))}
              </ul>
            )}
          </Card>

          <Card index={6} interactive={false}>
            <CardHeader title={t("connectors.sources.title")} />
            <ul className="space-y-3 text-sm">
              {(["kubernetes", "email", "slack", "webhook"] as const).map((id) => {
                const s = connectorSummary(id, facts);
                const name = CATALOG.find((x) => x.id === id)!.name;
                const dest = id === "kubernetes" ? "/kubernetes" : "/settings";
                return (
                  <li key={id}>
                    <Link href={dest} className="group flex items-center gap-3 rounded-xl p-1.5 transition-colors hover:bg-slate-50">
                      <ProviderMark id={id} size="sm" dim={s.state === "available"} />
                      <span className="min-w-0 flex-1">
                        <span className="block truncate font-medium">{name}</span>
                        <span className="block truncate text-xs text-muted">
                          {s.state === "available" ? t("connectors.not_set_up")
                            : id === "kubernetes" ? t("connectors.k8s.clusters", { count: s.count })
                            : t("connectors.channels.destinations", { count: s.count })}
                          {s.lastAt && id === "kubernetes" ? ` · ${timeAgo(s.lastAt)}` : ""}
                        </span>
                      </span>
                      {s.state !== "available" && <StateBadge state={s.state} label={t(`connection.state.${s.state}`)} />}
                    </Link>
                  </li>
                );
              })}
            </ul>
          </Card>
        </div>
      </div>

      <section className="space-y-4">
        <div className="flex flex-wrap items-center justify-between gap-3">
          <h2 className="text-lg font-semibold tracking-tight">{t("connectors.catalog.title")}</h2>
          <div role="tablist" className="flex flex-wrap gap-2">
            {CATEGORIES.map((c) => (
              <button key={c.id} role="tab" aria-selected={cat === c.id} onClick={() => setCat(c.id)}
                className={cn("rounded-full px-3 py-1 text-sm ring-1 ring-inset transition-all duration-200",
                  cat === c.id ? "bg-primary text-white ring-primary shadow-sm" : "bg-white text-slate-700 ring-line hover:bg-slate-50")}>
                {t(`connectors.cat.${c.id}`, undefined, c.label)}
              </button>
            ))}
          </div>
        </div>
        <div key={cat} className="grid gap-4 sm:grid-cols-2 xl:grid-cols-3">
          {items.map((item, i) => (
            <CatalogCard key={item.id} item={item} index={i} facts={facts} canConnect={canConnect} onConnect={(p) => setConnectAs(p)} />
          ))}
        </div>
      </section>

      <Drawer open={connectAs !== null && canConnect} onClose={() => setConnectAs(null)} title={t("connectors.drawer.title")} closeLabel={t("common.cancel")}>
        {connectAs && <ConnectionForm key={connectAs} initialProvider={connectAs} onDone={() => setConnectAs(null)} />}
      </Drawer>
    </div>
  );
}

/** A table row that opens a detail line under itself; the whole row is the button (and works from the keyboard). */
function ConnectionRow({ main, detail, open, onToggle, index }: { main: React.ReactNode; detail: React.ReactNode; open: boolean; onToggle: () => void; index: number }) {
  return (
    <>
      <tr className="row-in cursor-pointer border-t border-line/70 transition-colors hover:bg-slate-50/80" style={{ "--i": index } as CSSProperties}
        onClick={onToggle} onKeyDown={(e) => { if (e.key === "Enter" || e.key === " ") { e.preventDefault(); onToggle(); } }}
        tabIndex={0} aria-expanded={open}>
        {main}
        <td className="px-3 py-3 text-muted"><ChevronDown className={cn("size-4 transition-transform duration-300", open && "rotate-180")} /></td>
      </tr>
      <tr aria-hidden={!open}>
        <td colSpan={6} className="p-0">
          <div className="expand" data-open={open}><div>{detail}</div></div>
        </td>
      </tr>
    </>
  );
}

function CatalogCard({ item, index, facts, canConnect, onConnect }: { item: CatalogItem; index: number; facts: ConnectorFacts; canConnect: boolean; onConnect: (p: Provider) => void }) {
  const t = useT();
  const s = connectorSummary(item.id, facts);
  const planned = item.status === "planned", platform = item.status === "platform";
  const cloud = item.category === "cloud" && (item.provider === "aws" || item.provider === "gcp" || item.provider === "azure");
  const live = !planned && s.state !== "available";
  return (
    <Card index={index} interactive={false} className={cn("connector-card flex flex-col gap-4", planned && "bg-slate-50/60")}>
      <div className="flex items-start gap-3">
        <span data-mark className="transition-transform duration-300"><ProviderMark id={item.id} size="lg" dim={planned} /></span>
        <div className="min-w-0 flex-1">
          <div className="flex items-start justify-between gap-2">
            <p className="font-semibold leading-tight">{item.name}</p>
            {planned ? <Badge tone="stub">{item.phase ?? t("connectors.status.planned")}</Badge>
              : platform && !live ? <Badge tone="neutral">{t("connectors.status.platform")}</Badge>
              : live ? <StateBadge state={s.state as ConnectionState} label={t(`connection.state.${s.state}`)} />
              : <Badge tone="neutral">{t("connectors.status.available")}</Badge>}
          </div>
          <p className="mt-1 text-sm text-muted">{t(`connectors.item.${item.id}`, undefined, item.provides)}</p>
        </div>
      </div>
      <div className="mt-auto flex items-center justify-between text-sm">
        <span className="text-xs text-muted">
          {platform ? (live && s.lastAt ? `${t("connectors.status.platform")} · ${t("connectors.updated", { ago: timeAgo(s.lastAt) })}` : "")
            : live ? (cloud ? t("connectors.connected_count", { count: s.count }) : item.id === "kubernetes" ? t("connectors.k8s.clusters", { count: s.count }) : t("connectors.channels.destinations", { count: s.count })) : ""}
        </span>
        {planned || platform ? null : cloud && canConnect ? (
          <button onClick={() => onConnect(item.provider as Provider)} className="inline-flex items-center gap-1 font-medium text-primary transition-transform hover:translate-x-0.5">
            {s.count ? t("connectors.add_another") : t("connectors.connect_one")}<ArrowUpRight className="size-4" />
          </button>
        ) : item.id === "kubernetes" ? (
          <Link href="/kubernetes" className="inline-flex items-center gap-1 font-medium text-primary transition-transform hover:translate-x-0.5">{live ? t("connectors.manage") : t("connectors.set_up")}<ArrowUpRight className="size-4" /></Link>
        ) : item.category === "notifications" && canConnect ? (
          <Link href="/settings" className="inline-flex items-center gap-1 font-medium text-primary transition-transform hover:translate-x-0.5"><Bell className="size-4" />{live ? t("connectors.manage") : t("connectors.set_up")}</Link>
        ) : null}
      </div>
    </Card>
  );
}
