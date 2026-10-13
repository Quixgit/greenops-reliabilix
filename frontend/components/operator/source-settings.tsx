"use client";

import { useState } from "react";
import { ProviderMark } from "@/components/connectors/provider-mark";
import { IconChevronDown as ChevronDown, IconKey as KeyRound, IconWarning as TriangleAlert } from "@/components/ui/icons";
import { Card } from "@/components/ui/card";
import { ErrorState } from "@/components/ui/empty-state";
import { Skeleton } from "@/components/ui/skeleton";
import { useOperatorIntegrations, useOperatorMe } from "@/hooks/use-admin";
import type { AggSource, Aggregation } from "@/hooks/use-operator-extra";
import type { ManagedIntegration } from "@/lib/api/schemas";
import { cn } from "@/lib/utils";
import { IntegrationForm } from "./integrations";
import { Pill, type PillTone } from "./ui";

const FRESH: Record<AggSource["freshness"], { tone: PillTone; text: string }> = {
  fresh: { tone: "good", text: "fresh" }, aging: { tone: "warn", text: "aging" }, stale: { tone: "bad", text: "stale" }, none: { tone: "neutral", text: "no data" },
};
const ago = (iso: string | null) => {
  if (!iso) return "never";
  const h = (Date.now() - new Date(iso).getTime()) / 3_600_000;
  return h < 1 ? "under an hour ago" : h < 48 ? `${Math.round(h)} h ago` : `${Math.round(h / 24)} days ago`;
};

/** What the keys of a source say: whether it needs one and whether it is in place. */
function keyState(st: ManagedIntegration | undefined): { tone: PillTone; text: string; needsKey: boolean } {
  if (!st) return { tone: "neutral", text: "no settings", needsKey: false };
  const needed = st.fields.filter((f) => f.secret && f.required);
  if (needed.length === 0) return { tone: "neutral", text: "no key needed", needsKey: false };
  const missing = needed.filter((f) => !f.configured);
  return missing.length ? { tone: "warn", text: `${missing[0].label.toLowerCase()} not set`, needsKey: true } : { tone: "good", text: "key set", needsKey: true };
}

function SourceCard({ s, st, index, canEdit, position }: { s: AggSource; st: ManagedIntegration | undefined; index: number; canEdit: boolean; position: number }) {
  const [open, setOpen] = useState(false);
  const f = FRESH[s.freshness];
  const k = keyState(st);
  const usable = position > 0;
  return (
    <Card index={Math.min(index, 8)} interactive={false} className={cn("flex flex-col gap-3", !usable && "opacity-80")}>
      <div className="flex items-start gap-3">
        <ProviderMark id={s.id} size="md" dim={!usable} />
        <div className="min-w-0 flex-1">
          <h3 className="truncate text-[15px] font-semibold tracking-tight">{st?.name ?? s.label}</h3>
          <p className="mt-0.5 text-xs text-muted">{s.licence}</p>
        </div>
        <span className={cn("grid size-7 shrink-0 place-items-center rounded-full text-xs font-bold tabular-nums", usable ? "bg-leaf-soft text-leaf-deep" : "bg-slate-100 text-slate-400")} title={usable ? `Place ${position} in the priority` : "Not in the priority list: never used"}>{usable ? position : "–"}</span>
      </div>
      <div className="flex flex-wrap items-center gap-1.5">
        <Pill tone={s.computed ? "violet" : "info"}>{s.computed ? "computed from the generation mix" : "published intensity"}</Pill>
        <Pill tone="neutral">{s.granularity}</Pill>
        <Pill tone={f.tone} pulse={s.freshness === "fresh"}>{f.text}</Pill>
        <Pill tone={k.tone}>{k.text}</Pill>
      </div>
      <p className="text-xs text-muted">
        {s.newest ? `Newest reading ${ago(s.newest)}` : "Nothing stored yet"} · {s.regions} region{s.regions === 1 ? "" : "s"} · accepted age {s.max_age_hours} h
        {s.pinned_regions ? ` · pinned in ${s.pinned_regions}` : ""}
      </p>
      {k.needsKey && k.tone === "warn" && (
        <p className="flex items-start gap-2 rounded-xl bg-amber-50 px-3 py-2 text-xs text-[#77621a] ring-1 ring-amber-100">
          <TriangleAlert className="mt-0.5 size-3.5 shrink-0" aria-hidden />Without it this source is skipped; regions it would cover use the next source in the order.
        </p>
      )}
      {st && (
        <div className="mt-auto border-t border-line/70 pt-3">
          <button type="button" onClick={() => setOpen((o) => !o)} aria-expanded={open}
            className="flex w-full items-center justify-between gap-2 rounded-lg px-1 py-1 text-sm font-medium text-primary hover:bg-slate-50">
            <span className="inline-flex items-center gap-2"><KeyRound className="size-4" aria-hidden />{k.needsKey ? "Key and address" : "Address and test"}</span>
            <ChevronDown className={cn("size-4 transition-transform duration-300", open && "rotate-180")} aria-hidden />
          </button>
          {open && <div className="page-enter mt-3"><IntegrationForm st={st} canEdit={canEdit} /></div>}
        </div>
      )}
    </Card>
  );
}

/** Settings of every grid source in one place: kind, freshness, whether its key is in place, and the key itself, sealed. */
export function SourcesTab({ data }: { data: Aggregation }) {
  const ints = useOperatorIntegrations();
  const me = useOperatorMe();
  const canEdit = me.data?.role === "admin";
  if (ints.isLoading) return <Skeleton className="h-64" />;
  if (ints.isError) return <ErrorState />;
  const byId = new Map((ints.data ?? []).map((i) => [i.id, i]));
  const needKey = data.sources.filter((s) => keyState(byId.get(s.id)).tone === "warn").length;
  const pos = new Map(data.policy.priority.map((id, i) => [id, i + 1]));
  return (
    <div className="space-y-4">
      <p className="rise rounded-2xl bg-leaf-soft/50 px-4 py-3 text-sm text-slate-700 ring-1 ring-leaf/15">
        Keys and addresses of the grid-data sources. A key is stored sealed and is never shown again; a source without its key is skipped, not failed.
        The order between sources and their accepted age are on the Policy tab.
        {needKey > 0 && <b className="ml-1 text-[#77621a]">{needKey} source{needKey === 1 ? " needs" : "s need"} a key.</b>}
      </p>
      <div className="grid gap-4 lg:grid-cols-2 2xl:grid-cols-3">
        {data.sources.map((s, i) => <SourceCard key={s.id} s={s} st={byId.get(s.id)} index={i} canEdit={!!canEdit} position={pos.get(s.id) ?? 0} />)}
      </div>
    </div>
  );
}

const LEVEL: Record<"ok" | "differs" | "disagrees", { tone: PillTone; text: string }> = {
  ok: { tone: "good", text: "agree" }, differs: { tone: "warn", text: "differ" }, disagrees: { tone: "bad", text: "disagree" },
};

/** Where the sources of one region disagree, held against the independent monthly national figure. Nothing is combined here. */
export function DivergenceTab({ data }: { data: Aggregation }) {
  const rows = data.divergence;
  const flagged = rows.filter((r) => r.level !== "ok").length;
  return (
    <div className="space-y-4">
      <p className="rise rounded-2xl bg-slate-50 px-4 py-3 text-sm text-slate-700 ring-1 ring-line">
        Each figure is the <b>median of a source&apos;s hourly readings of the last 7 days</b> (one spike does not move it). Sources that differ by
        1.5× or more are marked; the figure a client sees is the one the policy takes (&quot;Used&quot;). The monthly national figure from Ember is an
        independent anchor: it is a country average, so a single region may differ from it for real reasons.
        {flagged > 0 ? <b className="ml-1 text-[#77621a]">{flagged} region{flagged === 1 ? "" : "s"} to look at.</b> : <b className="ml-1 text-leaf-deep">No region where sources disagree.</b>}
      </p>
      {rows.length === 0 ? (
        <Card interactive={false}><p className="text-sm text-muted">Nothing to compare yet: a region needs two sources with at least six hourly readings, or one source and a monthly national figure.</p></Card>
      ) : (
        <div className="glass overflow-hidden">
          <div className="thin-scroll overflow-x-auto">
            <table className="w-full min-w-[720px] border-separate border-spacing-0 text-sm">
              <thead>
                <tr className="text-left text-xs font-semibold uppercase tracking-wide text-slate-500">
                  {["Region", "Sources (gCO₂e/kWh, 7-day median)", "National references", "Spread", "Used", ""].map((h) => <th key={h} className="border-b border-line bg-slate-50/70 px-4 py-2.5">{h}</th>)}
                </tr>
              </thead>
              <tbody>
                {rows.map((r) => (
                  <tr key={r.region} className="align-top transition-colors hover:bg-slate-50/60">
                    <td className="border-b border-line/60 px-4 py-3">
                      <span className="font-medium">{r.region}</span>
                      <span className="mt-1 block text-[11px] text-muted">{r.in_use ? "in use" : "not used yet"}</span>
                    </td>
                    <td className="border-b border-line/60 px-4 py-3">
                      <ul className="space-y-1">
                        {r.sources.map((s) => (
                          <li key={s.id} className={cn("flex flex-wrap items-center gap-x-2", !s.fresh && "text-slate-400")}>
                            <span className="min-w-14 font-semibold tabular-nums">{Math.round(s.g_per_kwh)}</span>
                            <span>{s.label}</span>
                            {s.computed && <span className="text-[10px] uppercase tracking-wide text-violet-500">computed</span>}
                            {!s.fresh && <span className="text-[10px] uppercase tracking-wide">stale</span>}
                            {s.id === r.used && <span className="rounded-full bg-leaf-soft px-1.5 text-[10px] font-semibold text-leaf-deep">used</span>}
                            {s.vs_anchor_pct !== null && <span className="text-[11px] text-muted">{s.vs_anchor_pct >= 0 ? "+" : ""}{Math.round(s.vs_anchor_pct)}% vs national</span>}
                          </li>
                        ))}
                      </ul>
                    </td>
                    <td className="border-b border-line/60 px-4 py-3 tabular-nums">
                      {r.anchor || r.inventory ? <ul className="space-y-1">
                        {r.anchor && <li><span className="font-semibold">{Math.round(r.anchor.g_per_kwh)}</span> <span className="text-[11px] text-muted">monthly · {new Date(r.anchor.month).toLocaleDateString("en", { month: "short", year: "numeric" })}</span></li>}
                        {r.inventory && <li title="National power-sector emissions of the Climate TRACE inventory divided by the generation of that year (Ember). Direct emissions, so it sits below life-cycle figures.">
                          <span className="font-semibold">{Math.round(r.inventory.g_per_kwh)}</span> <span className="text-[11px] text-muted">inventory · {new Date(r.inventory.month).getUTCFullYear()}, direct</span></li>}
                      </ul> : <span className="text-muted">—</span>}
                    </td>
                    <td className="border-b border-line/60 px-4 py-3 tabular-nums">{r.sources.length > 1 ? `${r.ratio.toFixed(1)}×` : "—"}</td>
                    <td className="border-b border-line/60 px-4 py-3">{r.sources.find((s) => s.id === r.used)?.label ?? r.used}</td>
                    <td className="border-b border-line/60 px-4 py-3"><Pill tone={LEVEL[r.level].tone}>{LEVEL[r.level].text}</Pill></td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      )}
    </div>
  );
}
