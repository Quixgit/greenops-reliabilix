"use client";

import Link from "next/link";
import { useEffect, useMemo, useState } from "react";
import { useFinopsSummary, useServices } from "@/hooks/use-overview";
import { useT } from "@/i18n/provider";
import { cloudMeta } from "@/lib/clouds";
import { formatCarbon, formatCost, halfTrend, nf } from "@/lib/formatters";
import { share } from "@/lib/overview";
import { useUi } from "@/lib/store";
import { CloudMark, Ico, type DxIconName } from "../icons";
import { cloudId } from "@/lib/clouds";
import { Bar, Check, Delta, Empty, Panel, Skel, Spark, Switch, Tag } from "../ui";
import type { DxPrefs } from "./prefs";

const CATS = ["all", "compute", "storage", "database", "networking", "other"] as const;
const SHORT = 8;

/** Services ranked by cost with their share of cost and of emissions: the heavy emitters are tagged. */
export function ServicesPanel({ prefs, set }: { prefs: DxPrefs; set: <K extends keyof DxPrefs>(k: K, v: DxPrefs[K]) => void }) {
  const t = useT();
  const [cat, setCat] = useState<(typeof CATS)[number]>("all");
  const [all, setAll] = useState(false);
  const [open, setOpen] = useState<string | null>(null);
  const q = useServices(cat);
  const cur = useFinopsSummary().data?.currency || "USD";
  const focus = useUi((s) => s.focusService);
  const setFocus = useUi((s) => s.setFocusService);
  useEffect(() => { if (!focus) return; const id = setTimeout(() => setFocus(null), 4000); return () => clearTimeout(id); }, [focus, setFocus]);
  const data = useMemo(() => [...(q.data ?? [])].sort((a, b) => b.cost - a.cost), [q.data]);
  const totalCost = data.reduce((n, r) => n + r.cost, 0), totalCarbon = data.reduce((n, r) => n + r.carbon_kg_co2e, 0);
  const rows = useMemo(() => data.map((r) => {
    const sc = share(r.cost, totalCost), sk = share(r.carbon_kg_co2e, totalCarbon);
    return { r, sc, sk, heavy: sk > 4 && sk > sc * 1.6, trend: halfTrend(r.trend) };
  }), [data, totalCost, totalCarbon]);
  const filtered = prefs.heavyOnly ? rows.filter((x) => x.heavy) : rows;
  const shown = all ? filtered : filtered.slice(0, SHORT);
  const multi = new Set(data.map((r) => r.provider).filter(Boolean)).size > 1;

  return (
    <Panel icon="layers" tone="butter" title={t("ov2.svc.title")} hint={t("info.services")} index={6} flush
      tools={
        <>
          <div className="dx-seg is-sm is-icons" role="group" aria-label={t("ov2.svc.title")} style={{ ["--n" as string]: CATS.length, ["--i" as string]: CATS.indexOf(cat) }}>
            <span className="dx-seg-ind" aria-hidden />
            {CATS.map((c) => <button key={c} type="button" aria-pressed={c === cat} title={t(`ov2.cat.${c}`)} aria-label={t(`ov2.cat.${c}`)} onClick={() => { setCat(c); setOpen(null); }}><Ico name={c === "all" ? "layers" : (c as DxIconName)} size={16} /></button>)}
          </div>
          <Check checked={prefs.heavyOnly} onChange={(v) => set("heavyOnly", v)} label={t("ov2.svc.heavy")} />
        </>
      }>
      {q.isLoading ? <div className="dx-pb"><Skel h={240} /></div> : filtered.length === 0 ? <Empty icon="layers">{prefs.heavyOnly ? t("dx.svc.no_heavy", undefined, "No service emits much more than it costs.") : t("ov2.svc.empty")}</Empty> : (
        <>
          <div className="dx-th dx-svc">
            <span>{t("ov2.svc.service")}</span><span className="dx-svc-shares">{t("dx.svc.shares", undefined, "Cost · CO₂e share")}</span><span className="dx-num">{t("ov2.svc.cost")}</span><span className="dx-num">CO₂e</span><span className="dx-svc-trend">{t("ov2.svc.trend")}</span>
          </div>
          <ul className="dx-rows">
            {shown.map(({ r, sc, sk, heavy, trend }, i) => {
              const key = `${r.provider}|${r.service}`;
              const isOpen = open === key || focus === r.service;
              const icon = (CATS.includes(r.category as never) && r.category !== "all" ? r.category : "other") as DxIconName;
              return (
                <li key={key} className="dx-rise" style={{ ["--i" as string]: Math.min(i, 10) }}>
                  <div className={`dx-row dx-svc is-click ${isOpen ? "is-open" : ""}`} onClick={() => setOpen(isOpen ? null : key)}>
                    <span style={{ display: "flex", alignItems: "center", gap: 12, minWidth: 0 }}>
                      <span className={`dx-cat is-${r.category in { compute: 1, storage: 1, database: 1, networking: 1 } ? r.category : "other"}`}><Ico name={icon} size={18} /></span>
                      <span style={{ minWidth: 0 }}>
                        <span style={{ display: "block", fontWeight: 600, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }} title={r.service}>{r.service.replace(/^Amazon |^AWS /, "")}</span>
                        <span style={{ display: "flex", gap: 6, alignItems: "center", marginTop: 2 }}>
                          {multi && r.provider && <CloudMark id={cloudId(r.provider)} color={cloudMeta(r.provider).color} />}
                          {heavy && <Tag tone="peach" title={t("ov2.svc.heavy_tip")}>{t("ov2.svc.heavy")}</Tag>}
                        </span>
                      </span>
                    </span>
                    <span className="dx-svc-shares" style={{ display: "grid", gap: 5 }} aria-label={t("ov2.svc.shares")}>
                      <Bar pct={Math.max(2, sc)} color="var(--dx-cost)" index={i} thin />
                      <Bar pct={Math.max(2, sk)} color="var(--dx-carbon)" index={i + 1} thin />
                    </span>
                    <span className="dx-num">{formatCost(r.cost, cur)}</span>
                    <span className="dx-num">{formatCarbon(r.carbon_kg_co2e)}</span>
                    <span className="dx-svc-trend" style={{ display: "flex", alignItems: "center", gap: 10 }}><span style={{ width: 56 }}><Delta pct={trend} /></span><Spark values={r.trend} width={56} height={22} color="var(--dx-ink-2)" /></span>
                  </div>
                  {isOpen && (
                    <div className="dx-detail">
                      <div className="dx-fact"><small>{t("dx.share", undefined, "Share of cost")}</small><b>{nf(1).format(sc)}%</b></div>
                      <div className="dx-fact"><small>{t("dx.share_co2", undefined, "Share of emissions")}</small><b>{nf(1).format(sk)}%</b></div>
                      <div className="dx-fact"><small>{t("dx.cloud", undefined, "Cloud")}</small><b>{r.provider ? cloudMeta(r.provider).name : "—"}</b></div>
                      <div className="dx-fact" style={{ alignSelf: "end" }}><Link href="/recommendations" className="dx-btn is-sm">{t("dx.svc.recs", undefined, "Recommendations")}<Ico name="arrowRight" size={14} /></Link></div>
                    </div>
                  )}
                </li>
              );
            })}
          </ul>
          {filtered.length > SHORT && (
            <div className="dx-pb" style={{ borderTop: "1px solid var(--dx-line)", display: "flex", justifyContent: "space-between", alignItems: "center", paddingTop: 10, paddingBottom: 10 }}>
              <span style={{ color: "var(--dx-muted)" }}>{t("dx.showing", { n: shown.length, total: filtered.length }, "{n} of {total}")}</span>
              <Switch checked={all} onChange={setAll} label={t("dx.show_all", undefined, "Show all")} />
            </div>
          )}
        </>
      )}
    </Panel>
  );
}
