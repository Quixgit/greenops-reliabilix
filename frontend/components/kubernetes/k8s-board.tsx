"use client";

import Link from "next/link";
import { useMemo, useState, type CSSProperties } from "react";
import { Card, CardHeader } from "@/components/ui/card";
import { CountUp } from "@/components/ui/count-up";
import { EmptyState, ErrorState } from "@/components/ui/empty-state";
import { IconBolt, IconCluster, IconCopy, IconGauge, IconLeaf, IconSearch, IconWarning } from "@/components/ui/icons";
import { Info } from "@/components/ui/info";
import { Pagination, usePaged } from "@/components/ui/pagination";
import { Skeleton } from "@/components/ui/skeleton";
import { useK8sClusters, useK8sWorkloads, useRecommendations } from "@/hooks/use-overview";
import { useT } from "@/i18n/provider";
import type { K8sWorkload } from "@/lib/api/schemas";
import { carbonParts, nf, timeAgo } from "@/lib/formatters";
import { cn } from "@/lib/utils";

const LOW = 25;
const carbonText = (kg: number | null) => { if (kg === null) return "—"; const p = carbonParts(kg); return `${nf(p.n < 10 ? 2 : 1).format(p.n)} ${p.unit}`; };
const isLow = (w: K8sWorkload) => (w.cpu_utilization_pct ?? 100) < LOW || (w.memory_utilization_pct ?? 100) < LOW;

function Stat({ index, icon: Icon, tone, label, info, children, hint }: { index: number; icon: typeof IconLeaf; tone: string; label: string; info: string; children: React.ReactNode; hint?: string }) {
  return (
    <Card index={index} className="flex items-center gap-4 p-4">
      <Icon className={cn("size-11 shrink-0", tone)} strokeWidth={1.4} />
      <div className="min-w-0">
        <p className="flex items-center gap-1 text-[13px] font-medium text-slate-600">{label}<Info>{info}</Info></p>
        <p className="mt-0.5 text-2xl font-bold tracking-tight tabular-nums">{children}</p>
        {hint && <p className="truncate text-xs text-muted">{hint}</p>}
      </div>
    </Card>
  );
}

/** Used against requested: a bar that turns amber under a quarter, the first sign of an over-sized workload. */
function UseBar({ pct, label }: { pct: number | null | undefined; label: string }) {
  if (pct === null || pct === undefined) return <span className="text-muted" title={label}>—</span>;
  const low = pct < LOW;
  return (
    <span className="inline-flex items-center gap-2" title={low ? label : undefined}>
      <span className="h-1.5 w-14 overflow-hidden rounded-full bg-slate-100"><span className={cn("bar-x block h-full rounded-full", low ? "bg-pastel-amber" : "bg-leaf")} style={{ width: `${Math.max(3, Math.min(100, pct))}%` }} /></span>
      <span className={cn("w-9 text-right tabular-nums", low && "font-semibold text-[#a9741a]")}>{nf(0).format(pct)}%</span>
    </span>
  );
}

export function K8sStats() {
  const t = useT();
  const c = useK8sClusters();
  const w = useK8sWorkloads("");
  const recs = useRecommendations("open");
  const cl = c.data ?? [];
  const energy = cl.reduce((s, x) => s + x.energy_kwh, 0);
  const carbon = cl.every((x) => x.carbon_kg_co2e !== null) ? cl.reduce((s, x) => s + (x.carbon_kg_co2e ?? 0), 0) : null;
  const low = (w.data ?? []).filter(isLow).length;
  const open = (recs.data ?? []).filter((r) => r.type === "k8s_rightsizing").length;
  const p = carbon === null ? null : carbonParts(carbon);
  return (
    <div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-4">
      <Stat index={0} icon={IconCluster} tone="text-[#5b8fdc]" label={t("k8.stat.clusters")} info={t("k8.info.clusters")} hint={t("k8.stat.workloads", { count: cl.reduce((s, x) => s + x.workloads, 0) })}>
        {c.isLoading ? "–" : <CountUp value={cl.length} format={(n) => String(Math.round(n))} />}
      </Stat>
      <Stat index={1} icon={IconBolt} tone="text-[#d9a441]" label={t("k8.stat.energy")} info={t("k8.info.energy")} hint={t("k8.stat.energy_hint")}>
        {c.isLoading ? "–" : <><CountUp value={energy} format={(n) => nf(n < 100 ? 1 : 0).format(n)} /> <span className="text-sm font-medium text-slate-500">kWh</span></>}
      </Stat>
      <Stat index={2} icon={IconLeaf} tone="text-leaf-deep" label="CO₂e" info={t("k8.info.carbon")} hint={carbon === null && !c.isLoading ? t("k8.stat.carbon_wait") : undefined}>
        {c.isLoading ? "–" : p ? <><CountUp value={p.n} format={(n) => nf(1).format(n)} /> <span className="text-sm font-medium text-slate-500">{p.unit}</span></> : "—"}
      </Stat>
      <Stat index={3} icon={IconGauge} tone="text-[#8a78d8]" label={t("k8.stat.oversized")} info={t("k8.info.oversized")} hint={open > 0 ? t("k8.stat.open_recs", { count: open }) : t("k8.stat.low_hint")}>
        {w.isLoading ? "–" : <CountUp value={low} format={(n) => String(Math.round(n))} />}
      </Stat>
    </div>
  );
}

export function ClusterCards({ cluster, onPick }: { cluster: string; onPick: (c: string) => void }) {
  const t = useT();
  const q = useK8sClusters();
  const list = q.data ?? [];
  const max = Math.max(1, ...list.map((c) => c.energy_kwh));
  return (
    <section className="space-y-3">
      <h2 className="flex items-center gap-1.5 text-[15px] font-semibold tracking-tight">{t("k8.clusters")}<Info>{t("info.k8s_clusters")}</Info></h2>
      {q.isLoading ? <Skeleton className="h-32" /> : q.isError ? <ErrorState /> : (
        <ul className="grid gap-4 md:grid-cols-2 xl:grid-cols-3">
          {list.map((c, i) => {
            const age = (Date.now() - Date.parse(c.last_hour)) / 3_600_000;
            const fresh = age < 4;
            const on = cluster === c.cluster;
            return (
              <li key={c.cluster} className="rise" style={{ "--i": i } as CSSProperties}>
                <button type="button" onClick={() => onPick(on ? "" : c.cluster)} aria-pressed={on}
                  className={cn("group flex h-full w-full flex-col gap-4 rounded-2xl border bg-white p-5 text-left transition-all duration-300 hover:-translate-y-1 hover:shadow-[0_14px_32px_-14px_rgba(47,143,120,0.35)]", on ? "border-leaf shadow-[0_0_0_3px_rgba(111,195,168,0.18)]" : "border-line hover:border-leaf/50")}>
                  <span className="flex items-start gap-3">
                    <IconCluster className="size-10 shrink-0 text-[#5b8fdc] transition-transform duration-500 group-hover:scale-110 group-hover:-rotate-6" strokeWidth={1.4} />
                    <span className="min-w-0 flex-1">
                      <span className="block truncate text-base font-semibold">{c.cluster}</span>
                      <span className="block text-xs text-muted">{t("k8.cluster.meta", { ns: c.namespaces, wl: c.workloads })}</span>
                    </span>
                    <span className={cn("flex shrink-0 items-center gap-1.5 rounded-full px-2.5 py-1 text-[11px] font-semibold", fresh ? "bg-leaf-soft text-leaf-deep" : "bg-[#fdf3dc] text-[#a9741a]")}>
                      <span className={cn("size-1.5 rounded-full", fresh ? "animate-pulse bg-leaf" : "bg-pastel-amber")} />{fresh ? t("k8.live") : t("k8.stale")}
                    </span>
                  </span>
                  <span>
                    <span className="flex items-baseline justify-between"><span className="text-2xl font-bold tabular-nums tracking-tight">{nf(1).format(c.energy_kwh)} <span className="text-sm font-medium text-slate-500">kWh</span></span><span className="text-sm text-slate-600 tabular-nums">{carbonText(c.carbon_kg_co2e)} CO₂e</span></span>
                    <span className="mt-2 block h-1.5 overflow-hidden rounded-full bg-slate-100"><span className="bar-x block h-full rounded-full bg-leaf" style={{ width: `${Math.max(3, (c.energy_kwh / max) * 100)}%`, "--d": `${i * 70}ms` } as CSSProperties} /></span>
                  </span>
                  <span className="text-xs text-muted">{t("k8.last_data", { ago: timeAgo(c.last_hour) })}</span>
                </button>
              </li>
            );
          })}
        </ul>
      )}
    </section>
  );
}

/** Where the energy goes inside the chosen scope: namespaces and the heaviest workloads, side by side. */
export function EnergySplit({ cluster }: { cluster: string }) {
  const t = useT();
  const q = useK8sWorkloads(cluster);
  const list = q.data ?? [];
  const byNs = useMemo(() => {
    const m = new Map<string, number>();
    for (const w of list) m.set(w.namespace, (m.get(w.namespace) ?? 0) + w.energy_kwh);
    return [...m.entries()].sort((a, z) => z[1] - a[1]).slice(0, 6);
  }, [list]);
  const top = [...list].sort((a, z) => z.energy_kwh - a.energy_kwh).slice(0, 6);
  const total = list.reduce((s, w) => s + w.energy_kwh, 0) || 1;
  const bars = (rows: { name: string; sub?: string; v: number }[], tone: string) => (
    <ul className="space-y-3">
      {rows.map((r, i) => (
        <li key={r.name + (r.sub ?? "")}>
          <div className="flex items-baseline justify-between gap-3 text-sm"><span className="min-w-0 truncate font-medium">{r.name}{r.sub && <span className="ml-2 text-xs font-normal text-muted">{r.sub}</span>}</span><span className="shrink-0 tabular-nums text-slate-600">{nf(r.v < 10 ? 2 : 1).format(r.v)} kWh · {nf(0).format((r.v / total) * 100)}%</span></div>
          <span className="mt-1.5 block h-1.5 overflow-hidden rounded-full bg-slate-100"><span className={cn("bar-x block h-full rounded-full", tone)} style={{ width: `${Math.max(2, (r.v / total) * 100)}%`, "--d": `${i * 60}ms` } as CSSProperties} /></span>
        </li>
      ))}
    </ul>
  );
  if (q.isLoading) return <Skeleton className="h-56" />;
  if (list.length === 0) return null;
  return (
    <div className="grid gap-5 lg:grid-cols-2">
      <Card index={4} interactive={false}><CardHeader title={t("k8.by_ns")} info={t("k8.info.by_ns")} />{bars(byNs.map(([n, v]) => ({ name: n, v })), "bg-pastel-blue")}</Card>
      <Card index={5} interactive={false}><CardHeader title={t("k8.top")} info={t("k8.info.top")} />{bars(top.map((w) => ({ name: w.workload, sub: w.namespace, v: w.energy_kwh })), "bg-leaf")}</Card>
    </div>
  );
}

/** Right-sizing ideas the platform found from measured usage: current request, recommended request, and where to apply it. */
export function RightSizing() {
  const t = useT();
  const q = useRecommendations("open");
  const rows = (q.data ?? []).filter((r) => r.type === "k8s_rightsizing");
  if (q.isLoading || rows.length === 0) return null;
  const gib = (b?: string) => (b ? `${(Number(b) / 1024 ** 3).toFixed(1)} GiB` : "");
  return (
    <Card index={6} interactive={false}>
      <CardHeader icon={<IconGauge className="size-[18px] text-[#8a78d8]" />} title={t("k8.rs.title")} info={t("k8.info.rs")}
        action={<Link href="/recommendations" className="text-xs font-semibold text-leaf-deep hover:underline">{t("k8.rs.all")} →</Link>} />
      <ul className="divide-y divide-line/70">
        {rows.slice(0, 6).map((r) => {
          const d = r.details ?? {};
          return (
            <li key={r.id} className="flex flex-wrap items-center gap-x-6 gap-y-1 py-3 text-sm">
              <span className="min-w-0 flex-1 basis-56"><span className="block truncate font-semibold">{r.title}</span><span className="block text-xs text-muted">{r.current_region}</span></span>
              {d.cpu_request_recommended_cores && <span className="tabular-nums">CPU <b>{d.cpu_request_current_cores}</b> → <b className="text-leaf-deep">{d.cpu_request_recommended_cores}</b> <span className="text-muted">{t("k8.rs.cores")}</span></span>}
              {d.memory_request_recommended_bytes && <span className="tabular-nums">{t("k8.rs.memory")} <b>{gib(d.memory_request_current_bytes)}</b> → <b className="text-leaf-deep">{gib(d.memory_request_recommended_bytes)}</b></span>}
            </li>
          );
        })}
      </ul>
      <p className="mt-3 text-xs text-muted">{t("k8.rs.note")}</p>
    </Card>
  );
}

export function WorkloadTable({ cluster, onClear }: { cluster: string; onClear: () => void }) {
  const t = useT();
  const q = useK8sWorkloads(cluster);
  const [search, setSearch] = useState("");
  const [ns, setNs] = useState("");
  const [onlyLow, setOnlyLow] = useState(false);
  const [sort, setSort] = useState<"energy" | "carbon" | "cpu">("energy");
  const all = q.data ?? [];
  const namespaces = [...new Set(all.map((w) => w.namespace))].sort();
  const rows = all
    .filter((w) => (!ns || w.namespace === ns) && (!onlyLow || isLow(w)) && (!search || `${w.workload} ${w.namespace}`.toLowerCase().includes(search.toLowerCase())))
    .sort((a, z) => sort === "energy" ? z.energy_kwh - a.energy_kwh : sort === "carbon" ? (z.carbon_kg_co2e ?? -1) - (a.carbon_kg_co2e ?? -1) : (a.cpu_utilization_pct ?? 999) - (z.cpu_utilization_pct ?? 999));
  const pg = usePaged(rows, 10);
  const chip = (on: boolean) => cn("rounded-full px-3 py-1 text-xs font-semibold transition-all", on ? "bg-leaf-deep text-white" : "bg-slate-100 text-slate-600 hover:bg-leaf-soft hover:text-leaf-deep");
  return (
    <Card index={7} interactive={false}>
      <CardHeader title={cluster ? t("k8.wl.in", { cluster }) : t("k8.wl.title")} info={t("info.k8s_workloads")}
        action={cluster ? <button type="button" onClick={onClear} className="text-xs font-semibold text-leaf-deep hover:underline">{t("k8.wl.all_clusters")}</button> : undefined} />
      <div className="mb-3 flex flex-wrap items-center gap-3">
        <label className="relative">
          <IconSearch className="pointer-events-none absolute left-3 top-1/2 size-4 -translate-y-1/2 text-slate-400" />
          <input value={search} onChange={(e) => setSearch(e.target.value)} placeholder={t("k8.wl.search")} className="w-56 rounded-xl border border-line bg-white py-2 pl-9 pr-3 text-sm" aria-label={t("k8.wl.search")} />
        </label>
        <select value={ns} onChange={(e) => setNs(e.target.value)} aria-label={t("k8.wl.namespace")} className="rounded-xl border border-line bg-white px-3 py-2 text-sm">
          <option value="">{t("k8.wl.all_ns")}</option>{namespaces.map((n) => <option key={n} value={n}>{n}</option>)}
        </select>
        <button type="button" aria-pressed={onlyLow} onClick={() => setOnlyLow((v) => !v)} className={chip(onlyLow)}>{t("k8.wl.only_low")}</button>
        <div className="ml-auto flex gap-1.5" role="group" aria-label={t("k8.wl.sort")}>
          {(["energy", "carbon", "cpu"] as const).map((k) => <button key={k} type="button" aria-pressed={sort === k} onClick={() => setSort(k)} className={chip(sort === k)}>{t(`k8.sort.${k}`)}</button>)}
        </div>
      </div>
      {q.isLoading ? <Skeleton className="h-40" /> : q.isError ? <ErrorState /> : rows.length === 0 ? <p className="py-8 text-center text-sm text-muted">{t("k8.wl.none")}</p> : (
        <div className="overflow-x-auto">
          <table className="w-full min-w-[760px] text-left text-sm">
            <thead><tr className="text-xs uppercase tracking-wide text-muted">
              <th className="py-2 pr-3 font-medium">{t("k8.col.workload")}</th><th className="pr-3 font-medium">{t("k8.col.namespace")}</th><th className="pr-3 font-medium">{t("k8.col.kind")}</th>
              <th className="pr-3 text-right font-medium">{t("k8.col.energy")}</th><th className="pr-3 text-right font-medium">CO₂e</th><th className="pr-3 font-medium">CPU</th><th className="pr-3 font-medium">{t("k8.col.memory")}</th><th className="text-right font-medium">{t("k8.col.hours")}</th>
            </tr></thead>
            <tbody key={`${cluster}${ns}${sort}${onlyLow}${pg.page}`}>
              {pg.slice.map((w, i) => (
                <tr key={`${w.cluster}/${w.namespace}/${w.workload}`} className="row-in border-t border-line/60 transition-colors hover:bg-slate-50/70" style={{ "--i": i } as CSSProperties}>
                  <td className="py-2.5 pr-3 font-medium">{w.workload}{!cluster && <span className="ml-2 text-xs font-normal text-muted">{w.cluster}</span>}{isLow(w) && <IconWarning className="ml-1.5 inline size-3.5 text-[#d9a441]" />}</td>
                  <td className="pr-3">{w.namespace}</td><td className="pr-3 text-muted">{w.kind}</td>
                  <td className="pr-3 text-right tabular-nums">{nf(2).format(w.energy_kwh)} kWh</td>
                  <td className="pr-3 text-right tabular-nums" title={w.carbon_kg_co2e === null ? t("k8.no_grid") : undefined}>{carbonText(w.carbon_kg_co2e)}</td>
                  <td className="pr-3"><UseBar pct={w.cpu_utilization_pct} label={t("k8.low_tip")} /></td>
                  <td className="pr-3"><UseBar pct={w.memory_utilization_pct} label={t("k8.low_tip")} /></td>
                  <td className="text-right tabular-nums text-muted">{w.hours}{w.hours_without_grid_data > 0 && <span title={t("k8.no_grid")}> ({w.hours_without_grid_data}?)</span>}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
      <Pagination page={pg.page} pages={pg.pages} total={pg.total} size={pg.size} onPage={pg.setPage} onSize={pg.setSize} />
      <p className="mt-3 text-xs leading-relaxed text-muted">{t("k8.foot")} <Link href="/methodology" className="text-leaf-deep hover:underline">{t("k8.foot_link")}</Link></p>
    </Card>
  );
}

const SNIPPET = "PROM_URL=http://prometheus:9090 API_URL=<platform API> API_KEY=<agent key> PROJECT_ID=<project id> \\\nCLUSTER=<cluster name> REGION=<cloud region> scripts/kepler-push.sh";

export function K8sSetup() {
  const t = useT();
  const [done, setDone] = useState(false);
  return (
    <Card interactive={false}>
      <EmptyState icon={IconCluster} className="py-8" text={t("k8.setup.none")} />
      <ol className="mx-auto mt-2 max-w-xl space-y-3 text-sm">
        {[t("k8.setup.s1"), t("k8.setup.s2"), t("k8.setup.s3")].map((s, i) => (
          <li key={i} className="flex gap-3"><span className="grid size-6 shrink-0 place-items-center rounded-full bg-leaf-soft text-xs font-bold text-leaf-deep">{i + 1}</span><span className="pt-0.5 text-slate-700">{s}</span></li>
        ))}
      </ol>
      <div className="mx-auto mt-5 max-w-xl">
        <div className="mb-1 flex justify-end"><button type="button" onClick={() => { void navigator.clipboard?.writeText(SNIPPET); setDone(true); window.setTimeout(() => setDone(false), 1500); }} className="inline-flex items-center gap-1 rounded-md px-2 py-0.5 text-xs font-medium text-slate-500 hover:bg-slate-100"><IconCopy className="size-3.5" />{done ? t("au.copied") : t("au.copy")}</button></div>
        <pre className="overflow-x-auto rounded-xl bg-slate-900 px-4 py-3 font-mono text-xs text-slate-100">{SNIPPET}</pre>
        <p className="mt-3 text-center text-xs text-muted">{t("k8.setup.docs")} <Link href="/settings" className="text-leaf-deep hover:underline">{t("k8.setup.keys")}</Link></p>
      </div>
    </Card>
  );
}
