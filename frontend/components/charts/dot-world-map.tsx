"use client";

import { memo, useCallback, useEffect, useMemo, useRef, useState } from "react";
import { createPortal } from "react-dom";
import { IconArrowDown as ArrowDown, IconArrowLeft as ArrowLeft, IconArrowRight as ArrowRight, IconArrowUp as ArrowUp, IconLocate as Locate, IconExpand as Maximize2, IconMinus as Minus, IconPlus as Plus } from "@/components/ui/icons";
import { CloseButton } from "@/components/ui/close-button";
import type { RegionFootprint, RegionIntensity } from "@/lib/api/schemas";
import { landDots, project, MAP_H, MAP_W } from "@/lib/dot-grid";
import { REGION_GEO, REGION_LABEL, distanceKm, flag, regionProvider, rttMs } from "@/lib/regions";
import { useT } from "@/i18n/provider";
import { intensityColor, NEUTRAL, STOPS } from "@/lib/intensity-color";
import { formatCarbon, formatCost, nf } from "@/lib/formatters";
import { cn } from "@/lib/utils";


export type MapMode = "intensity" | "footprint" | "relocation";
type Marker = { region: string; x: number; y: number; city: string; cc: string; country: string; g: number | null; r: number; fp?: RegionFootprint };

const clamp = (v: number, lo: number, hi: number) => Math.max(lo, Math.min(hi, v));

/**
 * The land dots live on two canvases: the base is drawn once (a single paint per colour), the overlay only holds the
 * dots of the hovered country. The sweep-in is a CSS clip-path, so no frame is ever redrawn in JavaScript.
 */
const DotLayer = memo(function DotLayer({ byCountry, hoverCountry, pxWidth, sweep }: { byCountry: Map<string, number>; hoverCountry: string | null; pxWidth: number; sweep: boolean }) {
  const base = useRef<HTMLCanvasElement>(null);
  const hot = useRef<HTMLCanvasElement>(null);
  const dots = useMemo(() => landDots(), []);
  const colors = useMemo(() => dots.map((d) => { const v = byCountry.get(d.country); return v === undefined ? NEUTRAL : intensityColor(v); }), [dots, byCountry]);
  const W = Math.round(pxWidth), H = Math.round((pxWidth * MAP_H) / MAP_W), k = W / MAP_W;

  useEffect(() => {
    const cv = base.current;
    if (!cv) return;
    const ctx = cv.getContext("2d")!;
    ctx.clearRect(0, 0, W, H);
    const groups = new Map<string, number[]>();
    colors.forEach((c, i) => (groups.get(c) ?? groups.set(c, []).get(c)!).push(i));
    ctx.globalAlpha = 0.95;
    const r = 0.82 * k;
    for (const [c, idx] of groups) {
      ctx.fillStyle = c;
      ctx.beginPath();
      for (const i of idx) { const d = dots[i]; ctx.moveTo(d.x * k + r, d.y * k); ctx.arc(d.x * k, d.y * k, r, 0, 6.2832); }
      ctx.fill();
    }
  }, [dots, colors, W, H, k]);

  useEffect(() => {
    const cv = hot.current;
    if (!cv) return;
    const ctx = cv.getContext("2d")!;
    ctx.clearRect(0, 0, W, H);
    if (hoverCountry === null) return;
    const r = 1.2 * k;
    dots.forEach((d, i) => {
      if (d.country !== hoverCountry) return;
      ctx.fillStyle = colors[i];
      ctx.beginPath();
      ctx.arc(d.x * k, d.y * k, r, 0, 6.2832);
      ctx.fill();
    });
  }, [dots, colors, hoverCountry, W, H, k]);

  return (
    <div className={cn("absolute inset-0", sweep && "map-sweep")} aria-hidden>
      <canvas ref={base} width={W} height={H} className="absolute inset-0 size-full" />
      <canvas ref={hot} width={W} height={H} className="absolute inset-0 size-full" />
    </div>
  );
});

/** Dot-matrix world on a canvas, with markers and pop-over labels on top; in the big map you can zoom and drag. */
function DotMap({ regions, footprint, mode, big, selected, onSelect }: {
  regions: RegionIntensity[]; footprint: RegionFootprint[]; mode: MapMode; big: boolean; selected: string | null; onSelect: (r: string | null) => void;
}) {
  const { markers, byCountry, arcs } = useMemo(() => {
    const acc = new Map<string, number[]>();
    for (const r of regions) {
      const geo = REGION_GEO[r.region];
      if (geo) acc.set(geo.country, [...(acc.get(geo.country) ?? []), r.g_per_kwh]);
    }
    const gOf = new Map(regions.map((r) => [r.region, r.g_per_kwh]));
    const maxCost = Math.max(1, ...footprint.map((f) => f.cost));
    const ms: Marker[] = [];
    if (mode === "intensity") {
      for (const r of regions) {
        const geo = REGION_GEO[r.region];
        if (!geo) continue;
        const [x, y] = project(geo.lat, geo.lon);
        ms.push({ region: r.region, x, y, city: geo.city, cc: geo.cc, country: geo.country, g: r.g_per_kwh, r: 2 });
      }
    } else {
      for (const f of footprint) {
        const geo = REGION_GEO[f.region];
        if (!geo) continue;
        const [x, y] = project(geo.lat, geo.lon);
        const size = mode === "footprint" ? 1.6 + 4.4 * Math.sqrt(Math.max(0, f.cost) / maxCost) : 2.2;
        ms.push({ region: f.region, x, y, city: geo.city, cc: geo.cc, country: geo.country, g: f.g_per_kwh ?? gOf.get(f.region) ?? null, r: size, fp: f });
      }
    }
    const as: { from: Marker; to: [number, number]; key: string }[] = [];
    if (mode === "relocation") {
      for (const m of ms) {
        const alt = m.fp?.alternative, ag = alt ? REGION_GEO[alt.region] : undefined;
        if (alt && ag) as.push({ from: m, to: project(ag.lat, ag.lon), key: `${m.region}>${alt.region}` });
      }
    }
    return { markers: ms, byCountry: new Map([...acc].map(([c, v]) => [c, v.reduce((a, b) => a + b, 0) / v.length])), arcs: as };
  }, [regions, footprint, mode]);

  // Canvas resolution: the real width on screen, a little extra for zooming; never more than 2400 px.
  const [pxWidth, setPxWidth] = useState(big ? 1800 : 900);
  const [hover, setHover] = useState<string | null>(null);
  const hoverCountry = useMemo(() => {
    const id = hover ?? selected;
    return id ? REGION_GEO[id]?.country ?? null : null;
  }, [hover, selected]);

  // Zoom and pan are applied straight to the stage's transform (GPU, no React re-render per frame).
  const wrap = useRef<HTMLDivElement>(null);
  const stage = useRef<HTMLDivElement>(null);
  const view = useRef({ k: 1, x: 0, y: 0 });
  const [zoomed, setZoomed] = useState(false);
  const apply = useCallback((fly = false) => {
    const w = wrap.current, st = stage.current;
    if (!w || !st) return;
    const v = view.current, W = w.clientWidth, H = w.clientHeight;
    v.x = clamp(v.x, W - W * v.k, 0); v.y = clamp(v.y, H - H * v.k, 0);
    st.classList.toggle("map-fly", fly);
    st.style.transform = `translate3d(${v.x}px,${v.y}px,0) scale(${v.k})`;
    st.style.setProperty("--k", String(v.k));
    setZoomed(v.k > 1.01);
  }, []);
  const zoomAt = useCallback((k2: number, cx: number, cy: number, fly = false) => {
    const v = view.current, k = clamp(k2, 1, 6), r = k / v.k;
    v.x = cx - (cx - v.x) * r; v.y = cy - (cy - v.y) * r; v.k = k;
    apply(fly);
  }, [apply]);

  useEffect(() => {
    const w = wrap.current;
    if (!w || !big) return;
    let drag: { x: number; y: number; vx: number; vy: number } | null = null;
    const wheel = (e: WheelEvent) => {
      e.preventDefault();
      const r = w.getBoundingClientRect();
      zoomAt(view.current.k * Math.exp(-e.deltaY * 0.0016), e.clientX - r.left, e.clientY - r.top);
    };
    const down = (e: PointerEvent) => { if ((e.target as HTMLElement).closest("[data-marker],[data-map-ui]")) return; drag = { x: e.clientX, y: e.clientY, vx: view.current.x, vy: view.current.y }; w.setPointerCapture(e.pointerId); w.classList.add("map-grabbing"); };
    const move = (e: PointerEvent) => { if (!drag) return; view.current.x = drag.vx + e.clientX - drag.x; view.current.y = drag.vy + e.clientY - drag.y; apply(); };
    const up = () => { drag = null; w.classList.remove("map-grabbing"); };
    w.addEventListener("wheel", wheel, { passive: false });
    w.addEventListener("pointerdown", down); w.addEventListener("pointermove", move); w.addEventListener("pointerup", up); w.addEventListener("pointercancel", up);
    return () => { w.removeEventListener("wheel", wheel); w.removeEventListener("pointerdown", down); w.removeEventListener("pointermove", move); w.removeEventListener("pointerup", up); w.removeEventListener("pointercancel", up); };
  }, [big, apply, zoomAt]);

  // Selecting a region flies the camera to it.
  useEffect(() => {
    const w = wrap.current;
    if (!big || !w || !selected) return;
    const m = markers.find((x) => x.region === selected);
    if (!m) return;
    const W = w.clientWidth, H = w.clientHeight, k = Math.max(view.current.k, 2.4);
    view.current = { k, x: W / 2 - (m.x / MAP_W) * W * k, y: H / 2 - (m.y / MAP_H) * H * k };
    apply(true);
  }, [selected, big, markers, apply]);

  useEffect(() => {
    const w = wrap.current;
    if (!w) return;
    const dpr = Math.min(window.devicePixelRatio || 1, 2);
    setPxWidth(Math.round(Math.min(2400, Math.max(600, w.clientWidth * dpr * (big ? 1.8 : 1.15)))));
  }, [big]);

  const reset = () => { view.current = { k: 1, x: 0, y: 0 }; apply(true); };
  const pan = (dx: number, dy: number) => { const w = wrap.current; if (!w) return; view.current.x += dx * w.clientWidth * 0.25; view.current.y += dy * w.clientHeight * 0.25; apply(true); };
  const step = (f: number) => { const w = wrap.current; if (w) zoomAt(view.current.k * f, w.clientWidth / 2, w.clientHeight / 2, true); };

  return (
    <div ref={wrap} className={cn("map-wrap relative w-full overflow-hidden rounded-2xl", big && "cursor-grab")} style={{ aspectRatio: `${MAP_W} / ${MAP_H}` }}>
      <div ref={stage} className="map-stage absolute inset-0 origin-top-left" style={{ ["--k" as string]: 1 }}>
        <DotLayer byCountry={byCountry} hoverCountry={hoverCountry} pxWidth={pxWidth} sweep={!big} />
        <svg viewBox={`0 0 ${MAP_W} ${MAP_H}`} className="absolute inset-0 size-full overflow-visible" role="img" aria-label="Carbon intensity of the electricity grid by region">
          {arcs.map((a) => {
            const mx = (a.from.x + a.to[0]) / 2, my = Math.min(a.from.y, a.to[1]) - Math.hypot(a.from.x - a.to[0], a.from.y - a.to[1]) * 0.28;
            return (
              <g key={a.key}>
                <path d={`M${a.from.x},${a.from.y} Q${mx},${my} ${a.to[0]},${a.to[1]}`} fill="none" stroke="#8fb4f0" strokeWidth={0.7} strokeLinecap="round" className="map-arc" />
                <circle cx={a.to[0]} cy={a.to[1]} r={1.5} fill="#8fd4bf" stroke="#fff" strokeWidth={0.4} className="map-breathe" />
              </g>
            );
          })}
          {markers.map((m, i) => {
            const on = selected === m.region || hover === m.region;
            const fill = m.g === null ? "#b9c6dc" : intensityColor(m.g);
            return (
              <g key={m.region} data-marker className="map-marker cursor-pointer" style={{ transform: `translate(${m.x}px,${m.y}px) scale(calc(1 / var(--k, 1)))` }}
                onMouseEnter={() => setHover(m.region)} onMouseLeave={() => setHover(null)} onClick={() => onSelect(m.region)}>
                <circle r={m.r + 3} fill="transparent" />
                <circle r={m.r + 0.6} fill={fill} className="map-pulse" style={{ animationDelay: `${(i % 5) * 500}ms` }} />
                <g className="map-pop" style={{ animationDelay: `${300 + i * 60}ms` }}>
                  <circle r={m.r + (on ? 1.2 : 0.6)} fill="#fff" className="map-core" />
                  <circle r={m.r + (on ? 0.4 : -0.1)} fill={fill} stroke="rgba(15,27,51,.22)" strokeWidth={0.3} className="map-core" />
                  {selected === m.region && <circle r={m.r + 2.4} fill="none" stroke={fill} strokeWidth={0.5} strokeDasharray="1.6 1.6" className="map-ring" />}
                </g>
              </g>
            );
          })}
        </svg>
        {markers.map((m) => {
          const on = hover === m.region && selected !== m.region;
          if (!on && !(big && mode !== "intensity" && selected !== m.region)) return null;
          return (
            <div key={m.region} className="map-pill pointer-events-none absolute z-10 whitespace-nowrap rounded-xl border border-white/70 bg-white/85 px-2.5 py-1 text-[11px] font-semibold text-ink shadow-[0_8px_24px_rgba(31,52,110,.14)] "
              style={{ left: `${(m.x / MAP_W) * 100}%`, top: `${(m.y / MAP_H) * 100}%`, transform: "translate(-50%, -165%) scale(calc(1 / var(--k, 1)))", transformOrigin: "50% 100%" }}>
              <span className="mr-1.5" aria-hidden>{flag(m.cc)}</span>{m.city}
              <span className="ml-1.5 tabular-nums text-muted">
                {mode === "footprint" && m.fp ? formatCost(m.fp.cost) : mode === "relocation" && m.fp?.alternative ? `−${nf(0).format(m.fp.alternative.carbon_saving_pct)}%` : m.g === null ? "" : nf(0).format(m.g)}
              </span>
            </div>
          );
        })}
        {markers.filter((m) => m.region === selected).map((m) => (
          <MiniCard key={m.region} m={m} below={m.y < MAP_H * 0.5} compact={!big} onClose={() => onSelect(null)} />
        ))}
      </div>
      {big && (
        <div data-map-ui className="absolute bottom-3 right-3 z-20 flex flex-col items-end gap-2">
        <div className="grid grid-cols-3 gap-0.5 rounded-xl border border-white/70 bg-white/85 p-1 shadow-[0_8px_24px_rgba(31,52,110,.14)]">
          <span />
          <button type="button" aria-label="Pan up" onClick={() => pan(0, 1)} className="grid size-7 place-items-center rounded-lg text-muted transition hover:bg-white hover:text-ink"><ArrowUp className="size-3.5" /></button>
          <span />
          <button type="button" aria-label="Pan left" onClick={() => pan(1, 0)} className="grid size-7 place-items-center rounded-lg text-muted transition hover:bg-white hover:text-ink"><ArrowLeft className="size-3.5" /></button>
          <button type="button" aria-label="Reset view" onClick={reset} className="grid size-7 place-items-center rounded-lg text-muted transition hover:bg-white hover:text-ink"><Locate className="size-3.5" /></button>
          <button type="button" aria-label="Pan right" onClick={() => pan(-1, 0)} className="grid size-7 place-items-center rounded-lg text-muted transition hover:bg-white hover:text-ink"><ArrowRight className="size-3.5" /></button>
          <span />
          <button type="button" aria-label="Pan down" onClick={() => pan(0, -1)} className="grid size-7 place-items-center rounded-lg text-muted transition hover:bg-white hover:text-ink"><ArrowDown className="size-3.5" /></button>
          <span />
        </div>
        <div className="flex flex-col overflow-hidden rounded-xl border border-white/70 bg-white/85 shadow-[0_8px_24px_rgba(31,52,110,.14)]">
          <button type="button" aria-label="Zoom in" onClick={() => step(1.6)} className="grid size-8 place-items-center text-muted transition hover:bg-white hover:text-ink"><Plus className="size-4" /></button>
          <button type="button" aria-label="Zoom out" onClick={() => step(1 / 1.6)} className="grid size-8 place-items-center border-t border-line/70 text-muted transition hover:bg-white hover:text-ink"><Minus className="size-4" /></button>
        </div>
      </div>
      )}
    </div>
  );
}

/** The small window that opens on a marker: where it is, what the grid does there and what you have in it. */
function MiniCard({ m, below, compact, onClose }: { m: Marker; below: boolean; compact: boolean; onClose: () => void }) {
  const t = useT();
  const b = m.g === null ? null : m.g < 150 ? ["clean", "bg-leaf-soft text-leaf-deep"] : m.g < 300 ? ["moderate", "bg-pastel-amber/30 text-amber-800"] : m.g < 450 ? ["high", "bg-pastel-amber/40 text-amber-900"] : ["very_high", "bg-pastel-rose/30 text-rose-800"];
  return (
    <div data-map-ui className={cn("map-mini absolute z-30 rounded-2xl border border-white/80 bg-white/95 text-left shadow-[0_18px_44px_rgba(31,52,110,.22)] backdrop-blur", compact ? "w-[172px] p-2.5" : "w-[236px] p-3.5")}
      style={{ left: `${(m.x / MAP_W) * 100}%`, top: `${(m.y / MAP_H) * 100}%`, transform: `translate(${m.x < MAP_W * 0.16 ? "-12%" : m.x > MAP_W * 0.84 ? "-88%" : "-50%"}, ${below ? "12px" : "calc(-100% - 12px)"}) scale(calc(1 / var(--k, 1)))`, transformOrigin: below ? "50% 0" : "50% 100%" }}
      onClick={(e) => e.stopPropagation()}>
      <div className="flex items-start gap-2">
        <span className={cn("leading-none", compact ? "text-lg" : "text-2xl")} aria-hidden>{flag(m.cc)}</span>
        <div className="min-w-0 flex-1">
          <p className="truncate text-sm font-semibold leading-tight text-ink">{m.city}</p>
          <p className="truncate text-[11px] text-muted">{m.country} · {regionProvider(m.region)}</p>
        </div>
        <CloseButton onClick={onClose} label={t("map.close", undefined, "Close")} className={cn("-mr-1 -mt-1", compact ? "size-6" : "size-7")} />
      </div>
      {!compact && <p className="mt-1.5 font-mono text-[10px] text-slate-400">{m.region}</p>}
      <div className={cn("flex items-end justify-between", compact ? "mt-1.5" : "mt-2.5")}>
        <p className={cn("font-bold leading-none tracking-tight tabular-nums text-ink", compact ? "text-lg" : "text-2xl")}>{m.g === null ? "—" : nf(0).format(m.g)}<span className="ml-1 text-[9px] font-medium text-muted">g/kWh</span></p>
        {b && <span className={cn("rounded-full px-1.5 py-0.5 text-[9px] font-semibold", b[1])}>{t(`ov2.band.${b[0]}`, undefined, b[0])}</span>}
      </div>
      {compact ? (m.fp && <p className="mt-1.5 truncate border-t border-line/70 pt-1.5 text-[10px] tabular-nums text-muted">{formatCarbon(m.fp.carbon_kg_co2e)} · {formatCost(m.fp.cost)}</p>) : m.fp ? (
        <div className="mt-3 grid grid-cols-2 gap-2 border-t border-line/70 pt-2.5 text-center">
          <div><p className="text-[10px] text-muted">CO₂e</p><p className="text-xs font-semibold tabular-nums">{formatCarbon(m.fp.carbon_kg_co2e)}</p></div>
          <div><p className="text-[10px] text-muted">{t("map.cost", undefined, "Cost")}</p><p className="text-xs font-semibold tabular-nums">{formatCost(m.fp.cost)}</p></div>
        </div>
      ) : <p className="mt-2.5 border-t border-line/70 pt-2 text-[11px] text-muted">{t("ov2.reg.unused")}</p>}
    </div>
  );
}

function ModeSwitch({ mode, onChange, t, disabled }: { mode: MapMode; onChange: (m: MapMode) => void; t: ReturnType<typeof useT>; disabled: boolean }) {
  const items: [MapMode, string][] = [["intensity", t("map.layer_intensity", undefined, "Grid intensity")], ["footprint", t("map.layer_footprint", undefined, "My footprint")], ["relocation", t("map.layer_relocation", undefined, "Relocation")]];
  return (
    <div className="mt-3 inline-flex rounded-xl bg-slate-100 p-0.5 text-xs font-medium" role="tablist">
      {items.map(([k, label]) => (
        <button key={k} type="button" role="tab" aria-selected={mode === k} disabled={disabled && k !== "intensity"} onClick={() => onChange(k)}
          className={cn("rounded-[10px] px-3 py-1.5 transition", mode === k ? "bg-white text-ink shadow-sm" : "text-muted hover:text-ink", disabled && k !== "intensity" && "opacity-40")}>{label}</button>
      ))}
    </div>
  );
}

function Legend({ t, mode }: { t: ReturnType<typeof useT>; mode: MapMode }) {
  const grad = `linear-gradient(90deg,${STOPS.map(([g, c]) => `rgb(${c.join(",")}) ${(g / 600) * 100}%`).join(",")})`;
  return (
    <div className="mt-3 flex items-center gap-3 text-xs text-muted">
      <span>{t("map.clean", undefined, "Clean")}</span>
      <span className="h-2 w-40 rounded-full" style={{ background: grad }} />
      <span>{t("map.dirty", undefined, "High carbon")}</span>
      <span className="ml-auto hidden sm:inline">
        {mode === "footprint" ? t("map.size_cost", undefined, "Marker size: cost in the period · colour: grid intensity") : mode === "relocation" ? t("map.arcs", undefined, "Arrows: a cleaner region of the same cloud · operational effect only") : "gCO₂e/kWh · 0 – 600+"}
      </span>
    </div>
  );
}

function Detail({ r, fp, t }: { r: RegionIntensity; fp?: RegionFootprint; t: ReturnType<typeof useT> }) {
  const geo = REGION_GEO[r.region];
  return (
    <div className="rise rounded-2xl border border-line bg-white p-4" key={r.region}>
      <div className="flex items-center gap-2 text-[15px] font-semibold text-ink">
        {geo && <span className="text-xl" aria-hidden>{flag(geo.cc)}</span>}
        {REGION_LABEL[r.region] ?? r.region}
      </div>
      <dl className="mt-3 grid grid-cols-2 gap-x-4 gap-y-2 text-sm">
        <dt className="text-muted">{t("map.provider", undefined, "Cloud")}</dt><dd className="text-right font-medium">{regionProvider(r.region)}</dd>
        <dt className="text-muted">{t("map.region_id", undefined, "Region ID")}</dt><dd className="text-right font-mono text-xs leading-6">{r.region}</dd>
        <dt className="text-muted">{t("map.intensity", undefined, "Grid intensity")}</dt>
        <dd className="text-right font-semibold tabular-nums"><span className="mr-1.5 inline-block size-2.5 rounded-full align-middle" style={{ background: isNaN(r.g_per_kwh) ? NEUTRAL : intensityColor(r.g_per_kwh) }} />{isNaN(r.g_per_kwh) ? "—" : nf(0).format(r.g_per_kwh)} <span className="text-xs font-normal text-muted">gCO₂e/kWh</span></dd>
        <dt className="text-muted">{t("map.change", undefined, "Change")}</dt>
        <dd className="text-right tabular-nums">{r.change_pct === null ? "—" : `${r.change_pct <= 0 ? "↓" : "↑"} ${nf(1).format(Math.abs(r.change_pct))}%`}</dd>
        <dt className="text-muted">{t("map.measured", undefined, "Measured")}</dt><dd className="text-right text-xs leading-6">{r.at ? new Date(r.at).toLocaleString() : "—"}</dd>
      </dl>
      {fp && (
        <dl className="mt-3 grid grid-cols-2 gap-x-4 gap-y-2 border-t border-line/70 pt-3 text-sm">
          <dt className="text-muted">{t("map.cost", undefined, "Cost in period")}</dt><dd className="text-right font-medium tabular-nums">{formatCost(fp.cost)}</dd>
          <dt className="text-muted">{t("map.energy", undefined, "Energy")}</dt><dd className="text-right tabular-nums">{nf(0).format(fp.energy_kwh)} kWh</dd>
          <dt className="text-muted">{t("map.co2e", undefined, "Operational CO₂e")}</dt><dd className="text-right font-medium tabular-nums">{formatCarbon(fp.carbon_kg_co2e)}</dd>
        </dl>
      )}
      {fp?.alternative && (
        <div className="mt-3 rounded-xl bg-leaf-soft/70 p-3 text-xs leading-relaxed text-leaf-deep ring-1 ring-leaf-soft">
          <b>{REGION_LABEL[fp.alternative.region] ?? fp.alternative.region}</b> ({fp.alternative.region}): {nf(0).format(fp.alternative.g_per_kwh)} gCO₂e/kWh,
          {" "}−{nf(0).format(fp.alternative.carbon_saving_pct)}% grid intensity, up to ≈{formatCarbon(fp.alternative.carbon_saving_kg)} CO₂e in this period.
          {fp.alternative.cost_delta !== null && (
            <span className="mt-1 block font-medium">{t("map.cost_delta", { amount: `${fp.alternative.cost_delta > 0 ? "+" : "−"}${formatCost(Math.abs(fp.alternative.cost_delta))}` })}</span>
          )}
          {(() => { const km = distanceKm(r.region, fp.alternative.region); return km === null ? null : (
            <span className="mt-1 block">{t("map.distance", { km: nf(0).format(km), ms: rttMs(km) })}</span>); })()}
          <span className="mt-1 block text-leaf-deep/80">{t("map.checks", undefined, "Not yet checked: exact latency and service availability. Data residency is respected (project policy).")}</span>
        </div>
      )}
      <p className="mt-3 text-xs leading-relaxed text-muted">{t("map.note", undefined, "Low intensity does not mean low total emissions: those depend on the energy your workload uses there.")}</p>
    </div>
  );
}

/** World map of grid carbon intensity. A small card version that opens into a full-screen explorer. */
export function RegionMap({ regions, footprint = [], selected = null, onSelect }: {
  regions: RegionIntensity[]; footprint?: RegionFootprint[];
  /** Controlled selection: a click on a marker selects it for the page's own detail panel instead of opening the modal. */
  selected?: string | null; onSelect?: (region: string | null) => void;
}) {
  const t = useT();
  const [mode, setMode] = useState<MapMode>("intensity");
  const [open, setOpen] = useState(false);
  const [sel, setSel] = useState<string | null>(null);
  useEffect(() => {
    if (!open) return;
    const k = (e: KeyboardEvent) => e.key === "Escape" && setOpen(false);
    window.addEventListener("keydown", k);
    const prev = document.body.style.overflow;
    document.body.style.overflow = "hidden";
    return () => { window.removeEventListener("keydown", k); document.body.style.overflow = prev; };
  }, [open]);
  // Every region we can talk about: those with a grid reading, plus regions the account uses that have none yet (NaN).
  const rows = useMemo(() => {
    const have = new Set(regions.map((r) => r.region));
    const extra: RegionIntensity[] = footprint.filter((f) => !have.has(f.region))
      .map((f) => ({ region: f.region, g_per_kwh: f.g_per_kwh ?? NaN, change_pct: null, at: "" }));
    return [...regions, ...extra].sort((a, b) => (isNaN(a.g_per_kwh) ? 1e9 : a.g_per_kwh) - (isNaN(b.g_per_kwh) ? 1e9 : b.g_per_kwh));
  }, [regions, footprint]);
  const current = rows.find((r) => r.region === sel) ?? null;
  // one line per place: a city that holds regions of several clouds is one grid, so it is listed once with its clouds
  const groups = useMemo(() => {
    const m = new Map<string, { key: string; first: RegionIntensity; regions: string[]; clouds: string[] }>();
    for (const r of rows) {
      const key = REGION_LABEL[r.region] ?? r.region;
      const cloud = regionProvider(r.region);
      const e = m.get(key);
      if (e) { e.regions.push(r.region); if (!e.clouds.includes(cloud)) e.clouds.push(cloud); } else m.set(key, { key, first: r, regions: [r.region], clouds: [cloud] });
    }
    return [...m.values()];
  }, [rows]);

  return (
    <>
      <div className="relative">
        <DotMap regions={regions} footprint={footprint} mode={mode} big={false} selected={onSelect ? selected : null} onSelect={(r) => { if (onSelect) { onSelect(r); return; } if (r) { setSel(r); setOpen(true); } }} />
        <button type="button" onClick={() => setOpen(true)} aria-label={t("map.expand", undefined, "Expand map")}
          className="absolute right-2.5 top-2.5 grid size-8 place-items-center rounded-xl border border-line bg-white/90 text-muted shadow-sm transition hover:scale-105 hover:text-ink">
          <Maximize2 className="size-4" />
        </button>
        <Legend t={t} mode={mode} />
        <ModeSwitch mode={mode} onChange={setMode} t={t} disabled={footprint.length === 0} />
      </div>
      {open && createPortal(
        <div className="map-backdrop fixed inset-0 z-50 grid place-items-center bg-[#0f1b33]/40 p-3 sm:p-8" onClick={() => setOpen(false)}>
          <div className="map-modal flex max-h-[92vh] w-full max-w-[1280px] flex-col gap-4 overflow-auto rounded-3xl bg-white p-4 shadow-2xl sm:p-6" role="dialog" aria-modal onClick={(e) => e.stopPropagation()}>
            <div className="flex items-center justify-between">
              <h2 className="text-lg font-semibold tracking-tight">{t("map.title", undefined, "Carbon intensity by region")}</h2>
              <CloseButton onClick={() => setOpen(false)} label={t("map.close", undefined, "Close")} />
            </div>
            <div>
              <DotMap regions={regions} footprint={footprint} mode={mode} big selected={sel} onSelect={setSel} />
              <div className="flex flex-wrap items-center justify-between gap-x-6">
                <Legend t={t} mode={mode} />
                <ModeSwitch mode={mode} onChange={setMode} t={t} disabled={footprint.length === 0} />
              </div>
            </div>
            <div className="grid min-h-0 gap-4 lg:grid-cols-2">
              <div>
                {current ? <Detail r={current} fp={footprint.find((f) => f.region === current.region)} t={t} /> : <p className="rounded-2xl border border-dashed border-line p-4 text-sm text-muted">{t("map.pick", undefined, "Pick a region on the map or in the list.")}</p>}
              </div>
              <ul className="max-h-[22rem] divide-y divide-line/70 overflow-auto rounded-2xl border border-line">
                {groups.map((g) => (
                  <li key={g.key}>
                    <button type="button" onClick={() => setSel(g.first.region)} className={cn("flex w-full items-center gap-2 px-3 py-2 text-left text-sm transition hover:bg-slate-50", g.regions.includes(sel ?? "") && "bg-leaf-soft/60")}>
                      <span className="size-2.5 shrink-0 rounded-full" style={{ background: isNaN(g.first.g_per_kwh) ? NEUTRAL : intensityColor(g.first.g_per_kwh) }} />
                      <span className="min-w-0 flex-1 truncate">{REGION_LABEL[g.first.region] ?? g.first.region}</span>
                      <span className="flex shrink-0 gap-1">{g.clouds.map((c) => <span key={c} className="rounded-md bg-slate-100 px-1.5 py-0.5 text-[10px] font-medium text-slate-600">{c}</span>)}</span>
                      <span className="w-10 text-right tabular-nums">{isNaN(g.first.g_per_kwh) ? "—" : nf(0).format(g.first.g_per_kwh)}</span>
                    </button>
                  </li>
                ))}
              </ul>
            </div>
          </div>
        </div>,
        document.body)}
    </>
  );
}
