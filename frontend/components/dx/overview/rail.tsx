"use client";

import { useQueries } from "@tanstack/react-query";
import Link from "next/link";
import { useMemo, useState } from "react";
import { z } from "zod";
import { useDestinations } from "@/hooks/use-admin";
import { useActivity, useBudgets, useConnections, useEfficiency, useFinopsSummary, useK8sClusters, useOptimization, useProjects, useRecommendations } from "@/hooks/use-overview";
import { useT } from "@/i18n/provider";
import { api } from "@/lib/api/client";
import { cloudMeta } from "@/lib/clouds";
import { connectorSummary } from "@/lib/connectors";
import { formatCarbon, formatCost, timeAgo } from "@/lib/formatters";
import { CloudMark, Ico, type DxIconName } from "../icons";
import { Bar, Empty, Panel, Skel, Switch, Tag } from "../ui";

const policy = z.object({ allowed_regions: z.array(z.string()) });

/** The few things worth doing, biggest first, each one press from its page. */
export function NextActions() {
  const t = useT();
  const cur = useFinopsSummary().data?.currency || "USD";
  const q = useRecommendations("open");
  const opt = useOptimization().data;
  const [more, setMore] = useState(false);
  const sorted = useMemo(() => [...(q.data ?? [])].sort((a, b) => b.carbon_reduction_kg_month - a.carbon_reduction_kg_month), [q.data]);
  const top = sorted.slice(0, more ? 8 : 3);
  return (
    <Panel icon="recommendations" tone="peach" title={t("ov3.recs.title")} hint={t("info.recs")} index={3} flush>
      {opt && opt.open_count + opt.approved_count > 0 && (
        <div className="dx-pb" style={{ borderBottom: "1px solid var(--dx-line)", background: "var(--dx-band)", paddingTop: 10, paddingBottom: 10 }}>
          <b style={{ fontSize: 15 }}>−{formatCarbon(opt.potential_carbon_kg_month)} CO₂e/mo</b>
          <span style={{ color: "var(--dx-muted)", marginLeft: 8 }}>{opt.potential_saving_month > 0 ? t("ov.opt.saving", { amount: formatCost(opt.potential_saving_month, cur) }) : t("ov.opt.cost_unknown")}</span>
        </div>
      )}
      {q.isLoading ? <div className="dx-pb"><Skel h={110} /></div> : top.length === 0 ? <Empty icon="recommendations">{t("ov2.recs.empty")}</Empty> : (
        <ul className="dx-rows">
          {top.map((r) => (
            <li key={r.id}>
              <Link href="/recommendations" className="dx-row is-click" style={{ gridTemplateColumns: "minmax(0,1fr) auto" }}>
                <span style={{ minWidth: 0 }}>
                  <span style={{ display: "block", fontWeight: 600, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }} title={r.title}>{r.title}</span>
                  <span style={{ display: "flex", gap: 6, marginTop: 4 }}><Tag tone="line">{cloudMeta(r.provider).name}</Tag></span>
                </span>
                <span className="dx-num" style={{ lineHeight: 1.3 }}>
                  <b style={{ color: "var(--dx-good)" }}>−{r.carbon_reduction_kg_month.toFixed(1)} kg</b>
                  <span style={{ display: "block", color: "var(--dx-muted)", fontSize: 12 }}>
                    {r.estimated_cost_impact === null ? t("ov.rec.cost_waiting") : r.estimated_cost_impact === 0 ? t("ov.rec.cost_same") : `${r.estimated_cost_impact < 0 ? "−" : "+"}${formatCost(Math.abs(r.estimated_cost_impact))}/mo`}
                  </span>
                </span>
              </Link>
            </li>
          ))}
        </ul>
      )}
      <div className="dx-pb" style={{ borderTop: "1px solid var(--dx-line)", paddingTop: 10, paddingBottom: 10, display: "flex", justifyContent: "space-between", alignItems: "center", gap: 8 }}>
        {sorted.length > 3 ? <Switch checked={more} onChange={setMore} label={t("ov3.recs.more", { n: sorted.length - 3 })} /> : <span />}
        <Link href="/recommendations" className="dx-btn is-quiet is-sm">{t("ov3.recs.all")}</Link>
      </div>
    </Panel>
  );
}

/** What is missing before every number on the page is real. Each step is a box that ticks itself. */
export function Setup() {
  const t = useT();
  const [showDone, setShowDone] = useState(false);
  const conns = useConnections();
  const projects = useProjects();
  const budgets = useBudgets();
  const eff = useEfficiency();
  const dests = useDestinations();
  const pols = useQueries({ queries: (projects.data ?? []).map((p) => ({ queryKey: ["policy", p.id], queryFn: () => api(`projects/${p.id}/policy`, policy) })) });
  const steps = useMemo(() => {
    const synced = (conns.data ?? []).some((c) => !!c.last_sync_at);
    const regions = pols.length > 0 && pols.every((p) => p.data) && pols.some((p) => (p.data?.allowed_regions.length ?? 0) > 0);
    return [
      { id: "connect", icon: "cloud" as DxIconName, done: (conns.data?.length ?? 0) > 0, href: "/connectors" },
      { id: "sync", icon: "refresh" as DxIconName, done: synced, href: "/connectors" },
      { id: "regions", icon: "pin" as DxIconName, done: regions, href: "/settings#policies" },
      { id: "units", icon: "scale" as DxIconName, done: !!eff.data?.has_units, href: "/projects?units=1" },
      { id: "budget", icon: "budget" as DxIconName, done: (budgets.data?.length ?? 0) > 0, href: "/settings" },
      { id: "notify", icon: "alerts" as DxIconName, done: (dests.data ?? []).some((d) => d.enabled), href: "/settings#notifications" },
    ];
  }, [conns.data, pols, eff.data, budgets.data, dests.data]);
  const loading = conns.isLoading || projects.isLoading || eff.isLoading;
  const done = steps.filter((s) => s.done).length;
  if (!loading && done === steps.length) return null;
  const list = showDone ? steps : steps.filter((s) => !s.done);
  return (
    <Panel icon="list" tone="sky" title={t("ov2.setup.title")} hint={t("info.setup")} index={2} flush tools={<Tag tone="sky">{done}/{steps.length}</Tag>}>
      <div className="dx-pb" style={{ paddingBottom: 10 }}><Bar pct={(done / steps.length) * 100} color="var(--dx-navy-3)" /></div>
      {loading ? <div className="dx-pb"><Skel h={110} /></div> : (
        <div>
          {list.map((s) => (
            <Link key={s.id} href={s.href} className={`dx-task ${s.done ? "is-done" : ""}`} title={t(`ov2.setup.${s.id}_why`)}>
              <span className="dx-box" style={s.done ? { background: "var(--dx-navy)", borderColor: "var(--dx-navy)" } : undefined}>{s.done && <svg viewBox="0 0 24 24" fill="none" stroke="#fff" strokeWidth="3.2" strokeLinecap="square" style={{ strokeDashoffset: 0 }}><path d="M4 12.5l5 5L20 6.5" /></svg>}</span>
              <span className="dx-t-name" style={{ flex: 1 }}>{t(`ov2.setup.${s.id}`)}</span>
              <Ico name={s.icon} size={16} />
            </Link>
          ))}
          <div className="dx-pb" style={{ borderTop: "1px solid var(--dx-line)", paddingTop: 10, paddingBottom: 10 }}><Switch checked={showDone} onChange={setShowDone} label={t("dx.show_done", undefined, "Show completed")} /></div>
        </div>
      )}
    </Panel>
  );
}

const DOT: Record<string, string> = { synced: "var(--dx-good)", waiting: "var(--dx-warn)", failing: "var(--dx-bad)", attention: "var(--dx-bad)" };
const ACC = [{ key: "aws", label: "AWS" }, { key: "azure", label: "Azure" }, { key: "gcp", label: "Google Cloud" }, { key: "kubernetes", label: "Kubernetes" }];

export function Accounts() {
  const t = useT();
  const q = useConnections();
  const clusters = useK8sClusters();
  const facts = { connections: q.data ?? [], clusters: clusters.data ?? [], destinations: [] };
  return (
    <Panel icon="cloud" tone="sage" title={t("ov3.accounts.title")} hint={t("info.accounts")} index={4} flush>
      {q.isLoading ? <div className="dx-pb"><Skel h={110} /></div> : (
        <ul className="dx-rows">
          {ACC.map((c) => {
            const s = connectorSummary(c.key, facts);
            const on = s.state !== "available";
            const href = c.key === "kubernetes" ? "/kubernetes" : on ? "/connectors" : `/connectors?connect=${c.key}`;
            return (
              <li key={c.key}>
                <Link href={href} className="dx-row is-click" style={{ gridTemplateColumns: "auto minmax(0,1fr) auto" }}>
                  <CloudMark id={c.key} color={on ? cloudMeta(c.key).color : "var(--dx-line-strong)"} />
                  <span style={{ fontWeight: 600, color: on ? "var(--dx-ink)" : "var(--dx-muted)" }}>{c.label}</span>
                  <span style={{ display: "flex", alignItems: "center", gap: 6, color: "var(--dx-muted)", fontSize: 12.5 }}>
                    <i style={{ width: 8, height: 8, background: on ? DOT[s.state] : "var(--dx-line-strong)", display: "inline-block" }} />
                    {on ? (s.lastAt ? timeAgo(s.lastAt) : t(`connection.state.${s.state}`)) : t("ov3.accounts.connect")}
                  </span>
                </Link>
              </li>
            );
          })}
        </ul>
      )}
    </Panel>
  );
}

const ACT_ICON: Record<string, DxIconName> = {
  "project.": "projects", "cloud_connection.": "connectors", "cloud_sync.": "refresh", "report.": "reports", "recommendation.": "recommendations", "budget.": "budget",
};

export function Activity() {
  const t = useT();
  const q = useActivity();
  const [all, setAll] = useState(false);
  const rows = (q.data ?? []).slice(0, all ? 12 : 5);
  const icon = (a: string): DxIconName => (Object.entries(ACT_ICON).find(([k]) => a.startsWith(k))?.[1] ?? "clock");
  const label = (a: string) => t(`act.${a.replace(".", "_")}`, undefined, a.replace(/[._]/g, " "));
  return (
    <Panel icon="clock" tone="butter" title={t("act.title")} index={5} flush tools={(q.data?.length ?? 0) > 5 ? <Switch checked={all} onChange={setAll} label={t("dx.show_all", undefined, "Show all")} /> : undefined}>
      {q.isLoading ? <div className="dx-pb"><Skel h={110} /></div> : rows.length === 0 ? <Empty icon="clock">{t("act.empty")}</Empty> : (
        <ul className="dx-rows">
          {rows.map((a) => (
            <li key={a.id} className="dx-row" style={{ gridTemplateColumns: "auto minmax(0,1fr) auto" }}>
              <span className="dx-ico is-sand" style={{ width: 24, height: 24 }}><Ico name={icon(a.action)} size={15} /></span>
              <span style={{ minWidth: 0, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>{label(a.action)}</span>
              <time dateTime={a.at} style={{ color: "var(--dx-muted)", fontSize: 12 }}>{timeAgo(a.at)}</time>
            </li>
          ))}
        </ul>
      )}
    </Panel>
  );
}
