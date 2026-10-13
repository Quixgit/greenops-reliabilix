"use client";

import dynamic from "next/dynamic";
import Link from "next/link";
import { useMemo, useState } from "react";
import type { MapMode } from "@/components/charts/dot-world-map";
import { useFinopsSummary, useIntensityHistory, useRegionFootprint, useRegions } from "@/hooks/use-overview";
import { useT } from "@/i18n/provider";
import { formatCarbon, formatCost, nf } from "@/lib/formatters";
import { intensityColor } from "@/lib/intensity-color";
import { regionName } from "@/lib/regions";
import { Empty, Panel, Segmented, Skel, Spark, Tag } from "../ui";

const RegionMap = dynamic(() => import("@/components/charts/region-map").then((m) => m.RegionMap), { ssr: false, loading: () => <Skel h={300} /> });

function band(g: number | null): { k: string; tone: "sage" | "butter" | "peach" | "rose" | "line" } {
  if (g === null) return { k: "dx.band.unknown", tone: "line" };
  if (g < 150) return { k: "ov2.band.clean", tone: "sage" };
  if (g < 300) return { k: "ov2.band.moderate", tone: "butter" };
  if (g < 450) return { k: "ov2.band.high", tone: "peach" };
  return { k: "ov2.band.very_high", tone: "rose" };
}

/** The grid under your regions: a map, the regions that carry your footprint, and the one you picked in detail. */
export function RegionsPanel() {
  const t = useT();
  const regions = useRegions();
  const fp = useRegionFootprint();
  const cur = useFinopsSummary().data?.currency || "USD";
  const [mode, setMode] = useState<MapMode>("intensity");
  const [sel, setSel] = useState<string | null>(null);
  const rows = useMemo(() => {
    const fps = new Map((fp.data ?? []).map((r) => [r.region, r]));
    return (regions.data ?? []).map((r) => ({ region: r.region, g: r.g_per_kwh as number | null, change: r.change_pct, fp: fps.get(r.region) }));
  }, [regions.data, fp.data]);
  const used = useMemo(() => rows.filter((r) => r.fp && (r.fp.cost > 0 || r.fp.carbon_kg_co2e > 0)).sort((a, b) => (b.fp!.carbon_kg_co2e - a.fp!.carbon_kg_co2e)), [rows]);
  const cleanest = useMemo(() => [...rows].filter((r) => r.g !== null).sort((a, b) => a.g! - b.g!).slice(0, 3), [rows]);
  const current = rows.find((r) => r.region === sel) ?? used[0] ?? cleanest[0] ?? null;
  const hist = useIntensityHistory(current?.region);
  const b = band(current?.g ?? null);
  const series = (hist.data?.points ?? []).map((p) => p.g_per_kwh);

  return (
    <Panel icon="pin" tone="sage" title={t("ov.reg.title")} hint={t("info.regions")} index={7} flush
      tools={<><Segmented small label={t("dx.map_mode", undefined, "Map")} value={mode === "relocation" ? "intensity" : mode} onChange={(v) => setMode(v as MapMode)} options={[{ value: "intensity", label: t("map.layer_intensity") }, { value: "footprint", label: t("map.layer_footprint") }]} />
        <Link href="/carbon" className="dx-btn is-quiet is-sm">{t("ov2.split.details")}</Link></>}>
      {regions.isLoading ? <div className="dx-pb"><Skel h={300} /></div> : rows.length === 0 ? <Empty icon="globe">{t("dx.reg.empty", undefined, "No grid data yet.")}</Empty> : (
        <div className="dx-split" style={{ gap: 0 }}>
          <div style={{ padding: 16, minWidth: 0 }}>
            <RegionMap limitMs={60} regions={regions.data!} footprint={fp.data ?? []} selected={current?.region ?? null} onSelect={setSel} mode={mode} onModeChange={setMode} />
            <div style={{ display: "flex", flexWrap: "wrap", gap: 8, marginTop: 12, alignItems: "center" }}>
              <span style={{ fontSize: 11, letterSpacing: "0.1em", textTransform: "uppercase", color: "var(--dx-muted)", fontWeight: 600 }}>{t("ov2.reg.yours")}</span>
              {used.length === 0 ? <span style={{ color: "var(--dx-muted)" }}>{t("ov.reg.no_resources")}</span> : used.slice(0, 4).map((r) => (
                <button key={r.region} type="button" aria-pressed={current?.region === r.region} onClick={() => setSel(r.region)} className="dx-btn is-sm" style={current?.region === r.region ? { background: "var(--dx-sand)", borderColor: "var(--dx-line-strong)" } : undefined}>
                  <span className="dx-swatch" style={{ background: r.g === null ? "var(--dx-line-strong)" : intensityColor(r.g) }} />{regionName(r.region)}
                </button>
              ))}
            </div>
          </div>
          <aside style={{ borderLeft: "1px solid var(--dx-line)", padding: 16, display: "grid", gap: 14, alignContent: "start", background: "var(--dx-band)" }}>
            {current && (
              <>
                <div>
                  <div style={{ display: "flex", justifyContent: "space-between", gap: 8, alignItems: "flex-start" }}>
                    <div style={{ minWidth: 0 }}><b style={{ fontSize: 15 }}>{regionName(current.region)}</b><div style={{ color: "var(--dx-muted)", fontSize: 12 }}>{current.region}</div></div>
                    <Tag tone={b.tone}>{t(b.k, undefined, "—")}</Tag>
                  </div>
                  <div style={{ display: "flex", alignItems: "baseline", gap: 8, marginTop: 10 }}>
                    <span style={{ fontSize: 30, fontWeight: 600, lineHeight: 1 }}>{current.g === null ? "—" : nf(0).format(current.g)}</span>
                    <span style={{ color: "var(--dx-muted)" }}>gCO₂e/kWh</span>
                  </div>
                  {series.length > 1 && <div style={{ marginTop: 10 }}><Spark values={series} width={260} height={40} color="var(--dx-carbon)" /></div>}
                </div>
                {current.fp && (
                  <div style={{ display: "grid", gridTemplateColumns: "repeat(3, minmax(0, 1fr))", gap: 8 }}>
                    <div className="dx-fact"><small>CO₂e</small><b>{formatCarbon(current.fp.carbon_kg_co2e)}</b></div>
                    <div className="dx-fact"><small>{t("ov3.trend.cost")}</small><b>{formatCost(current.fp.cost, cur)}</b></div>
                    <div className="dx-fact"><small>{t("dx.energy", undefined, "Energy")}</small><b>{nf(0).format(current.fp.energy_kwh)} kWh</b></div>
                  </div>
                )}
                {current.fp?.alternative && (
                  <div style={{ border: "1px solid var(--dx-line)", background: "var(--dx-surface)", padding: 12 }}>
                    <div style={{ fontSize: 11, letterSpacing: "0.1em", textTransform: "uppercase", color: "var(--dx-muted)", fontWeight: 600 }}>{t("ov2.pl.title")}</div>
                    <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", gap: 8, marginTop: 6 }}>
                      <b>{regionName(current.fp.alternative.region)}</b>
                      <Tag tone="sage">−{nf(0).format(current.fp.alternative.carbon_saving_pct)}%</Tag>
                    </div>
                    <div style={{ color: "var(--dx-muted)", fontSize: 12.5, marginTop: 4 }}>
                      {t("ov2.reg.move_detail", { kg: formatCarbon(current.fp.alternative.carbon_saving_kg) })}
                      {current.fp.alternative.latency_ms != null && <> · ≈ {current.fp.alternative.latency_ms} ms</>}
                    </div>
                  </div>
                )}
              </>
            )}
            {cleanest.length > 0 && (
              <div>
                <div style={{ fontSize: 11, letterSpacing: "0.1em", textTransform: "uppercase", color: "var(--dx-muted)", fontWeight: 600, marginBottom: 6 }}>{t("ov2.reg.cleanest_now")}</div>
                {cleanest.map((r) => (
                  <button key={r.region} type="button" className="dx-opt" style={{ padding: "6px 0" }} onClick={() => setSel(r.region)}>
                    <span className="dx-swatch" style={{ background: intensityColor(r.g!) }} /><span>{regionName(r.region)}</span><span style={{ marginLeft: "auto", color: "var(--dx-muted)" }}>{nf(0).format(r.g!)}</span>
                  </button>
                ))}
              </div>
            )}
          </aside>
        </div>
      )}
    </Panel>
  );
}
