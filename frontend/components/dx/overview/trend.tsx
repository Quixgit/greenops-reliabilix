"use client";

import Link from "next/link";
import { useEffect, useMemo, useRef, useState, type CSSProperties } from "react";
import { MonthlyHistoryBody } from "@/components/dashboard/monthly-history";
import { useCloudRows } from "@/components/dashboard/cloud-costs";
import { useAnomalies, useFinopsSummary, useRegionFootprint, useTrend } from "@/hooks/use-overview";
import { useT } from "@/i18n/provider";
import { cloudId, cloudMeta } from "@/lib/clouds";
import { formatCarbon, formatCost, nf } from "@/lib/formatters";
import { intensityColor } from "@/lib/intensity-color";
import { movingAverage, share } from "@/lib/overview";
import { regionName } from "@/lib/regions";
import { CloudMark } from "../icons";
import { Bar, Check, Empty, Panel, Segmented, Skel, Switch, Tag } from "../ui";
import type { DxPrefs } from "./prefs";

const H = 300, P = { l: 56, r: 52, t: 14, b: 28 };

function niceMax(v: number): number {
  if (v <= 0) return 1;
  const p = Math.pow(10, Math.floor(Math.log10(v))), n = v / p;
  return (n <= 1 ? 1 : n <= 2 ? 2 : n <= 2.5 ? 2.5 : n <= 5 ? 5 : 10) * p;
}

/** Cost as columns, CO₂e as a line on its own axis, with a crosshair, the spikes marked and a 7-day average if asked. */
export function TrendPanel({ prefs, set }: { prefs: DxPrefs; set: <K extends keyof DxPrefs>(k: K, v: DxPrefs[K]) => void }) {
  const t = useT();
  const q = useTrend();
  const cost = useFinopsSummary();
  const anomalies = useAnomalies();
  const cur = cost.data?.currency || "USD";
  const [view, setView] = useState<"daily" | "monthly">("daily");
  const [showCost, setShowCost] = useState(true);
  const [showCarbon, setShowCarbon] = useState(true);
  const [hover, setHover] = useState<number | null>(null);
  const box = useRef<HTMLDivElement>(null);
  const [W, setW] = useState(640);
  useEffect(() => {
    const el = box.current;
    if (!el || typeof ResizeObserver === "undefined") return;
    const ro = new ResizeObserver(([e]) => setW(Math.max(320, Math.round(e.contentRect.width))));
    ro.observe(el);
    return () => ro.disconnect();
  });
  const pts = useMemo(() => q.data ?? [], [q.data]);
  const n = pts.length;
  const costs = useMemo(() => pts.map((p) => p.cost), [pts]);
  const carbons = useMemo(() => pts.map((p) => p.carbon_kg_co2e), [pts]);
  const avg = useMemo(() => movingAverage(costs), [costs]);
  const maxC = niceMax(Math.max(0, ...costs)), maxK = niceMax(Math.max(0, ...carbons));
  const iw = W - P.l - P.r, ih = H - P.t - P.b;
  const slot = n > 0 ? iw / n : iw;
  const x = (i: number) => P.l + slot * i + slot / 2;
  const yC = (v: number) => H - P.b - (v / maxC) * ih;
  const yK = (v: number) => H - P.b - (v / maxK) * ih;
  const lineK = carbons.map((v, i) => `${i ? "L" : "M"}${x(i).toFixed(1)},${yK(v).toFixed(1)}`).join(" ");
  const lineAvg = avg.map((v, i) => `${i ? "L" : "M"}${x(i).toFixed(1)},${yC(v).toFixed(1)}`).join(" ");
  const spikeDays = useMemo(() => new Set((anomalies.data ?? []).map((a) => a.day.slice(0, 10))), [anomalies.data]);
  const ticks = [0, 0.25, 0.5, 0.75, 1];
  const labels = n > 1 ? Array.from({ length: Math.min(6, n) }, (_, k) => Math.round((k / (Math.min(6, n) - 1)) * (n - 1))) : [0];
  const onMove = (e: React.MouseEvent) => {
    const r = box.current?.getBoundingClientRect();
    if (!r || n < 1) return;
    const px = ((e.clientX - r.left) / r.width) * W;
    setHover(Math.max(0, Math.min(n - 1, Math.floor((px - P.l) / slot))));
  };
  const h = hover !== null ? pts[hover] : null;
  const fmtDay = (d: string) => new Date(d).toLocaleDateString(undefined, { month: "short", day: "numeric", timeZone: "UTC" });

  return (
    <Panel icon="forecast" tone="sky" title={t("ov2.trend.title")} hint={t("info.trend")} index={4}
      tools={<Segmented small label={t("ov2.trend.title")} value={view} onChange={setView} options={[{ value: "daily", label: t("ov3.trend.daily") }, { value: "monthly", label: t("dx.monthly", undefined, "Monthly") }]} />}>
      {view === "monthly" ? <MonthlyHistoryBody /> : q.isLoading ? <Skel h={300} /> : n === 0 ? <Empty icon="forecast">{t("ov2.trend.empty")}</Empty> : (
        <>
          <div className="dx-legend" style={{ marginBottom: 12 }}>
            <Check checked={showCost} onChange={(v) => (v || showCarbon) && setShowCost(v)} color="var(--dx-cost)" label={t("ov3.trend.cost")} />
            <Check checked={showCarbon} onChange={(v) => (v || showCost) && setShowCarbon(v)} color="var(--dx-carbon)" label="CO₂e" />
            <span className="dx-sep" />
            <Switch checked={prefs.avg} onChange={(v) => set("avg", v)} label={t("dx.avg_short", undefined, "7-day avg.")} title={t("ov2.trend.avg7")} />
            <Switch checked={prefs.spikes} onChange={(v) => set("spikes", v)} label={t("dx.spikes", undefined, "Spikes")} title={t("ov2.trend.spike")} />
          </div>
          <div className="dx-chart" ref={box} onMouseMove={onMove} onMouseLeave={() => setHover(null)}>
            <svg viewBox={`0 0 ${W} ${H}`} width={W} height={H} role="img" aria-label={t("ov2.trend.title")}>
              {ticks.map((k) => {
                const y = H - P.b - k * ih;
                return (
                  <g key={k}>
                    <line className="dx-gridline" x1={P.l} x2={W - P.r} y1={y} y2={y} />
                    {showCost && <text className="dx-axis" x={P.l - 8} y={y + 3.5} textAnchor="end">{formatCost(maxC * k, cur)}</text>}
                    {showCarbon && <text className="dx-axis" x={W - P.r + 8} y={y + 3.5}>{nf(maxK < 5 ? 1 : 0).format(maxK * k)} kg</text>}
                  </g>
                );
              })}
              {showCost && costs.map((v, i) => (
                <rect key={pts[i].day} className={`dx-col ${hover !== null && hover !== i ? "is-dim" : ""}`} style={{ "--i": i } as CSSProperties}
                  x={x(i) - Math.max(1.5, slot * 0.34)} width={Math.max(3, slot * 0.68)} y={yC(v)} height={Math.max(0, H - P.b - yC(v))} />
              ))}
              {showCost && prefs.avg && <path className="dx-line-avg" d={lineAvg} />}
              {showCarbon && <path className="dx-line-carbon" pathLength={1} d={lineK} />}
              {showCost && prefs.spikes && pts.map((p, i) => spikeDays.has(p.day.slice(0, 10)) && <rect key={`s${p.day}`} x={x(i) - 4} y={yC(p.cost) - 11} width="8" height="8" fill="var(--dx-warn)" />)}
              {hover !== null && <line x1={x(hover)} x2={x(hover)} y1={P.t} y2={H - P.b} stroke="var(--dx-ink)" strokeWidth="1" strokeDasharray="3 3" />}
              {labels.map((i) => <text key={i} className="dx-axis" x={x(i)} y={H - 8} textAnchor="middle">{fmtDay(pts[i].day)}</text>)}
            </svg>
            {h && hover !== null && (
              <div className="dx-tip" style={{ left: `${Math.min(78, Math.max(0, ((x(hover)) / W) * 100 - 8))}%`, top: 6 }}>
                <b>{fmtDay(h.day)}</b>
                <div><span>{t("ov3.trend.cost")}</span><span>{formatCost(h.cost, cur)}</span></div>
                <div><span>CO₂e</span><span>{formatCarbon(h.carbon_kg_co2e)}</span></div>
                {spikeDays.has(h.day.slice(0, 10)) && <div style={{ color: "var(--dx-warn)", fontWeight: 600 }}><span>{t("ov2.trend.spike")}</span></div>}
              </div>
            )}
          </div>
        </>
      )}
    </Panel>
  );
}

type By = "cloud" | "region";

/** Where it goes: by cloud or by region, two bars each (cost and emissions), a row opens into the facts. */
export function BreakdownPanel() {
  const t = useT();
  const [chosen, setBy] = useState<By | null>(null);
  const [open, setOpen] = useState<string | null>(null);
  const clouds = useCloudRows();
  // with one cloud, a split by cloud is a single line: show the regions until the person chooses
  const by: By = chosen ?? (clouds.rows.length > 1 ? "cloud" : "region");
  const fp = useRegionFootprint();
  const cur = clouds.cur;

  const [allRegions, setAllRegions] = useState(false);
  const sortedRegions = useMemo(() => [...(fp.data ?? [])].sort((a, b) => b.cost - a.cost), [fp.data]);
  const regionRows = allRegions ? sortedRegions : sortedRegions.slice(0, 6);
  const totalCost = regionRows.reduce((n, r) => n + r.cost, 0), totalCarbon = regionRows.reduce((n, r) => n + r.carbon_kg_co2e, 0);
  const cloudCarbon = clouds.rows.reduce((n, r) => n + r.carbon, 0);

  return (
    <Panel icon="layers" tone="peach" title={t("dx.breakdown", undefined, "Breakdown")} hint={t("info.split")} index={5}
      toolbar={<Segmented small label="By" value={by} onChange={(v) => { setBy(v); setOpen(null); }} options={[{ value: "cloud", label: t("ov2.split.by_provider") }, { value: "region", label: t("ov2.split.by_region") }]} />} flush>
      {by === "cloud" ? (
        clouds.loading ? <div className="dx-pb"><Skel h={160} /></div> : clouds.rows.length === 0 ? <Empty icon="cloud">{t("ov2.empty.waiting")}</Empty> : (
          <ul className="dx-rows">
            {clouds.rows.map((r, i) => {
              const m = cloudMeta(r.provider);
              const isOpen = open === r.provider;
              return (
                <li key={r.provider}>
                  <div className={`dx-row is-click ${isOpen ? "is-open" : ""}`} style={{ gridTemplateColumns: "minmax(0,1fr) auto" }} onClick={() => setOpen(isOpen ? null : r.provider)}>
                    <div style={{ minWidth: 0 }}>
                      <div style={{ display: "flex", alignItems: "center", gap: 8 }}><CloudMark id={cloudId(r.provider)} color={m.color} /><b style={{ fontWeight: 600 }}>{m.name}</b>{r.behind > 2 && <Tag tone="butter">{t("dx.stale", undefined, "Behind")}</Tag>}</div>
                      <div style={{ display: "grid", gap: 4, marginTop: 8 }}>
                        <Bar pct={r.share} color="var(--dx-cost)" index={i} thin />
                        <Bar pct={share(r.carbon, cloudCarbon)} color="var(--dx-carbon)" index={i + 1} thin />
                      </div>
                    </div>
                    <div className="dx-num" style={{ lineHeight: 1.3 }}><b>{formatCost(r.cost, cur)}</b><div style={{ color: "var(--dx-muted)", fontSize: 12.5 }}>{formatCarbon(r.carbon)}</div></div>
                  </div>
                  {isOpen && (
                    <div className="dx-detail">
                      <div className="dx-fact"><small>{t("dx.share", undefined, "Share of cost")}</small><b>{nf(0).format(r.share)}%</b></div>
                      <div className="dx-fact"><small>{t("dx.days", undefined, "Days with data")}</small><b>{r.daysWithData}</b></div>
                      {r.forecast !== null && <div className="dx-fact"><small>{t("ov2.fin.forecast")}</small><b>{formatCost(r.forecast, cur)}</b></div>}
                      {r.foreign && r.native[0] && <div className="dx-fact"><small>{t("dx.billed", undefined, "Billed in")}</small><b>{r.native.map((n) => `${nf(0).format(n.amount)} ${n.currency}`).join(" + ")}</b></div>}
                    </div>
                  )}
                </li>
              );
            })}
          </ul>
        )
      ) : (
        fp.isLoading ? <div className="dx-pb"><Skel h={160} /></div> : regionRows.length === 0 ? <Empty icon="pin">{t("ov.reg.no_resources")}</Empty> : (
          <ul className="dx-rows">
            {regionRows.map((r, i) => {
              const isOpen = open === r.region;
              return (
                <li key={`${r.provider}|${r.region}`}>
                  <div className={`dx-row is-click ${isOpen ? "is-open" : ""}`} style={{ gridTemplateColumns: "minmax(0,1fr) auto" }} onClick={() => setOpen(isOpen ? null : r.region)}>
                    <div style={{ minWidth: 0 }}>
                      <div style={{ display: "flex", alignItems: "center", gap: 8 }}>
                        <span className="dx-swatch" style={{ background: r.g_per_kwh === null ? "var(--dx-line-strong)" : intensityColor(r.g_per_kwh) }} aria-hidden />
                        <b style={{ fontWeight: 600, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>{regionName(r.region)}</b>
                      </div>
                      <div style={{ display: "grid", gap: 4, marginTop: 8 }}>
                        <Bar pct={share(r.cost, totalCost)} color="var(--dx-cost)" index={i} thin />
                        <Bar pct={share(r.carbon_kg_co2e, totalCarbon)} color="var(--dx-carbon)" index={i + 1} thin />
                      </div>
                    </div>
                    <div className="dx-num" style={{ lineHeight: 1.3 }}><b>{formatCost(r.cost, cur)}</b><div style={{ color: "var(--dx-muted)", fontSize: 12.5 }}>{formatCarbon(r.carbon_kg_co2e)}</div></div>
                  </div>
                  {isOpen && (
                    <div className="dx-detail">
                      <div className="dx-fact"><small>{t("dx.intensity", undefined, "Grid intensity")}</small><b>{r.g_per_kwh === null ? "—" : `${nf(0).format(r.g_per_kwh)} g/kWh`}</b></div>
                      <div className="dx-fact"><small>{t("dx.energy", undefined, "Energy")}</small><b>{nf(0).format(r.energy_kwh)} kWh</b></div>
                      <div className="dx-fact"><small>{t("dx.cloud", undefined, "Cloud")}</small><b>{cloudMeta(r.provider).name}</b></div>
                      {r.alternative && <div className="dx-fact"><small>{t("ov2.pl.cleaner_option")}</small><b>{regionName(r.alternative.region)} · −{nf(0).format(r.alternative.carbon_saving_pct)}%</b></div>}
                    </div>
                  )}
                </li>
              );
            })}
          </ul>
        )
      )}
      <div className="dx-pb" style={{ borderTop: "1px solid var(--dx-line)", display: "flex", justifyContent: "space-between", alignItems: "center", gap: 8, flexWrap: "wrap", paddingTop: 10, paddingBottom: 10 }}>
        {by === "region" && sortedRegions.length > 6 ? <Switch checked={allRegions} onChange={setAllRegions} label={t("dx.show_all", undefined, "Show all")} /> : <span className="dx-legend"><span className="dx-check"><span className="dx-swatch" style={{ background: "var(--dx-cost)" }} />{t("ov3.trend.cost")}</span><span className="dx-check"><span className="dx-swatch" style={{ background: "var(--dx-carbon)" }} />CO₂e</span></span>}
        <Link href="/carbon" className="dx-btn is-quiet is-sm">{t("ov2.split.details")}</Link>
      </div>
    </Panel>
  );
}
