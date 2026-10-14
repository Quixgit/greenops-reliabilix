"use client";

import { useEffect, useState } from "react";
import { useRegions } from "@/hooks/use-overview";
import { content } from "@/content/schema";
import { useT } from "@/i18n/provider";
import { regionName } from "@/lib/regions";
import { cn } from "@/lib/utils";
import { LeafOutline } from "@/components/ui/brand-icons";

/**
 * Soft green waves along the foot of the promo card. Each layer is one seamless two-period curve twice the card's width that slides by
 * exactly one period, so the loop has no seam; the layers run at different speeds and a few specks rise out of the water.
 */
function PromoWaves() {
  // one period is 200 wide; the curve is drawn for two so that sliding by half of it repeats exactly
  const curve = (y: number, a: number) => `M0 ${y} C 33 ${y - a}, 67 ${y - a}, 100 ${y} S 167 ${y + a} 200 ${y} S 267 ${y - a} 300 ${y} S 367 ${y + a} 400 ${y}`;
  const layers = [
    { y: 26, a: 11, cls: "pw-a", fill: "url(#pwf1)", line: "url(#pwl1)", w: 1.1 },
    { y: 32, a: 9, cls: "pw-b", fill: "url(#pwf2)", line: "url(#pwl2)", w: 0.9 },
    { y: 38, a: 7, cls: "pw-c", fill: "url(#pwf3)", line: "url(#pwl1)", w: 0.8 },
  ];
  return (
    <div className="pointer-events-none absolute inset-x-0 bottom-0 h-16 overflow-hidden rounded-b-2xl" aria-hidden>
      <div className="absolute inset-x-0 bottom-0 h-10 bg-gradient-to-t from-[#2fb985]/20 to-transparent" />
      {layers.map((l) => (
        <svg key={l.cls} viewBox="0 0 400 60" preserveAspectRatio="none" className={cn("absolute bottom-0 left-0 h-full w-[200%]", l.cls)}>
          <path d={`${curve(l.y, l.a)} L400 60 L0 60 Z`} fill={l.fill} />
          <path d={curve(l.y, l.a)} fill="none" stroke={l.line} strokeWidth={l.w} vectorEffect="non-scaling-stroke" />
        </svg>
      ))}
      <svg width="0" height="0" className="absolute">
        <defs>
          <linearGradient id="pwf1" x1="0" y1="0" x2="0" y2="1"><stop offset="0" stopColor="#5be3b1" stopOpacity=".34" /><stop offset="1" stopColor="#1f8f6e" stopOpacity="0" /></linearGradient>
          <linearGradient id="pwf2" x1="0" y1="0" x2="0" y2="1"><stop offset="0" stopColor="#4cc3d8" stopOpacity=".26" /><stop offset="1" stopColor="#2f9bb5" stopOpacity="0" /></linearGradient>
          <linearGradient id="pwf3" x1="0" y1="0" x2="0" y2="1"><stop offset="0" stopColor="#9bf5d3" stopOpacity=".2" /><stop offset="1" stopColor="#3ddc97" stopOpacity="0" /></linearGradient>
          <linearGradient id="pwl1" x1="0" y1="0" x2="1" y2="0"><stop offset="0" stopColor="#3ddc97" stopOpacity=".15" /><stop offset=".5" stopColor="#9bf5d3" stopOpacity=".95" /><stop offset="1" stopColor="#3ddc97" stopOpacity=".15" /></linearGradient>
          <linearGradient id="pwl2" x1="0" y1="0" x2="1" y2="0"><stop offset="0" stopColor="#7de0e8" stopOpacity=".1" /><stop offset=".5" stopColor="#7de0e8" stopOpacity=".8" /><stop offset="1" stopColor="#7de0e8" stopOpacity=".1" /></linearGradient>
        </defs>
      </svg>
      {[14, 34, 52, 71, 88].map((x, n) => <span key={x} className="pw-speck absolute bottom-1 size-[3px] rounded-full bg-[#9bf5d3]" style={{ left: `${x}%`, animationDelay: `${n * 1.3}s`, animationDuration: `${5 + (n % 3)}s` }} />)}
    </div>
  );
}

/** The card at the foot of the menu: the brand line, the three verbs and a live fact, one after another. */
export function PromoCard() {
  const t = useT();
  const regions = useRegions();
  const cleanest = regions.data?.[0]; // the API lists regions from the cleanest grid up
  const slides = [
    { key: "tagline", title: content.sidebarPromo.title, text: content.sidebarPromo.text },
    { key: "verbs", title: t("promo.verbs_title", undefined, "Measure. Optimize. Automate."), text: t("promo.verbs_text", undefined, "From the bill to the grid: one place for cost and carbon.") },
    { key: "impact", title: t("promo.impact_title", undefined, "Your cloud, your impact."), text: t("promo.impact_text", undefined, "Every service, region and workload, accounted for.") },
    ...(cleanest ? [{ key: "grid", title: t("promo.grid_title", { region: regionName(cleanest.region) }, `Cleanest grid now: ${regionName(cleanest.region)}`), text: t("promo.grid_text", { g: Math.round(cleanest.g_per_kwh) }, `${Math.round(cleanest.g_per_kwh)} gCO₂e/kWh. Where you run decides how clean your cloud is.`) }] : []),
  ];
  const [i, setI] = useState(0);
  const [paused, setPaused] = useState(false);
  useEffect(() => {
    if (paused) return;
    const id = window.setInterval(() => setI((n) => n + 1), 5200);
    return () => window.clearInterval(id);
  }, [paused]);
  const slide = slides[i % slides.length];
  return (
    <div onMouseEnter={() => setPaused(true)} onMouseLeave={() => setPaused(false)}
      className="relative mx-3 mb-3 overflow-hidden rounded-2xl bg-white/[0.06] p-3 pb-9 ring-1 ring-white/10 transition-colors duration-300 hover:bg-white/10">
      <PromoWaves />
      <div className="relative flex items-start gap-2.5">
        <span className="grid size-8 shrink-0 place-items-center rounded-lg bg-white/10 text-leaf"><LeafOutline className="size-[18px]" /></span>
        <div key={slide.key} className="promo-in min-w-0 flex-1">
          <p className="text-[13px] font-semibold leading-snug text-white">{slide.title}</p>
          <p className="mt-1 text-xs leading-snug text-slate-400">{slide.text}</p>
        </div>
      </div>
      <div className="relative mt-2.5 flex justify-center gap-1" aria-hidden>
        {slides.map((s2, n) => <span key={s2.key} className={cn("h-1 rounded-full transition-all duration-500", n === i % slides.length ? "w-4 bg-leaf" : "w-1.5 bg-white/20")} />)}
      </div>
    </div>
  );
}

