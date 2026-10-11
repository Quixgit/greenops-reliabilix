"use client";

import { useEffect, useState } from "react";
import { useRegions } from "@/hooks/use-overview";
import Link from "next/link";
import { usePathname } from "next/navigation";
import { content } from "@/content/schema";
import { useT } from "@/i18n/provider";
import { regionName } from "@/lib/regions";
import { useUi } from "@/lib/store";
import { BUILD_VERSION } from "@/lib/version";
import { cn } from "@/lib/utils";
import { LeafOutline } from "@/components/ui/brand-icons";
import { Logo } from "./logo";
import { MAIN_NAV } from "./nav";
import { CollapseToggle } from "./collapse-toggle";
import { GearMenu } from "./gear-menu";
import { PlanCard } from "./plan-card";

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
function PromoCard() {
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

export function Sidebar({ isOperator = false }: { isOperator?: boolean }) {
  const path = usePathname();
  const { navOpen, setNavOpen, navCollapsed } = useUi();
  const t = useT();
  const items = MAIN_NAV; // the operator console is in the gear menu: it is for the platform's administrator, not part of the client's workspace
  const slug = (href: string) => href.slice(1).replace(/\W+/g, "_");
  // The collapsed menu is a desktop state: on a phone the drawer is always the full menu.
  const slim = navCollapsed;
  return (
    <>
      {navOpen && <button aria-label="Close menu" className="fixed inset-0 z-30 bg-slate-900/40 backdrop-blur-[2px] lg:hidden" onClick={() => setNavOpen(false)} />}
      <aside className={cn(
        "sb-aside fixed inset-y-0 left-0 z-40 flex w-[236px] flex-col overflow-hidden bg-gradient-to-b from-navy to-navy-2 text-slate-300 shadow-[4px_0_24px_rgba(8,15,30,0.18)] lg:translate-x-0",
        navOpen ? "translate-x-0" : "-translate-x-full")}>
        <div className={cn("flex items-center pb-4 pt-5", slim ? "flex-col gap-3 px-0" : "justify-between pl-5 pr-3")}>
          <Link href="/overview" onClick={() => setNavOpen(false)} className={cn("flex items-center gap-3", slim && "justify-center")} title="Reliabilix">
            <Logo className="h-11 w-9 shrink-0" />
            {!slim && <span className="leading-tight"><span className="block text-[20px] font-bold tracking-tight text-white">Reliabilix</span><span className="block text-xs font-medium uppercase tracking-[0.18em] text-leaf/80">Systems</span></span>}
          </Link>
          <CollapseToggle className="hidden lg:grid" />
        </div>
        <nav aria-label="Main" className={cn("thin-scroll flex min-h-0 flex-1 flex-col gap-1 pt-1", slim ? "nav-slim items-center px-2" : "px-2 pl-3")}>
          {items.map(({ href, label, icon: Icon }) => {
            const active = path === href || path.startsWith(`${href}/`);
            const text = t(`nav.${slug(href)}`, undefined, label);
            return (
              <Link key={href} href={href} onClick={() => setNavOpen(false)} aria-current={active ? "page" : undefined} aria-label={slim ? text : undefined} title={slim ? text : undefined}
                className={cn("nav-link flex items-center rounded-xl text-[15px] font-medium", slim ? "size-11 justify-center" : "gap-3.5 px-3.5 py-3", active ? "bg-navy-active text-white" : "text-slate-300")}>
                <Icon className={cn("nav-icon shrink-0", slim ? "size-[22px]" : "size-[19px]")} strokeWidth={1.8} />{!slim && text}
              </Link>
            );
          })}
        </nav>
        <GearMenu collapsed={slim} isOperator={isOperator} />
        <PlanCard collapsed={slim} />
        {!slim && <div className="[@media(max-height:940px)]:hidden"><PromoCard /></div>}
        {!slim && (
          <div className="px-5 pb-4 text-[11px] leading-snug text-slate-500">
            <p className="font-medium text-slate-400">Reliabilix GreenOps Platform</p>
            <p className="flex items-center gap-1.5" title={`Platform release ${BUILD_VERSION.full}`}>
              v{BUILD_VERSION.number}
              {BUILD_VERSION.stage !== "stable" && <span className="rounded bg-white/10 px-1.5 py-px text-[10px] font-semibold tracking-wide text-leaf">{BUILD_VERSION.full.split("-").slice(1).join("-") || BUILD_VERSION.stage}</span>}
            </p>
          </div>
        )}
        {slim && <p className="pb-3 text-center text-[10px] font-semibold text-slate-500" title={`Platform release ${BUILD_VERSION.full}`}>v{BUILD_VERSION.number}</p>}
      </aside>
    </>
  );
}
