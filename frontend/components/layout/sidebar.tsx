"use client";

import { IconShield as ShieldCheck } from "@/components/ui/icons";
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
import { NAV } from "./nav";

/** Flowing curves for the promo card. Static, smooth shapes that gently breathe: there is no repeating tile, so no seam. */
function Waves() {
  return (
    <svg viewBox="0 0 212 130" preserveAspectRatio="none" className="pointer-events-none absolute inset-x-0 bottom-0 h-[118px] w-full" aria-hidden>
      <defs>
        <linearGradient id="wf1" x1="0" y1="1" x2="1" y2="0"><stop offset="0" stopColor="#2f8f78" stopOpacity="0" /><stop offset=".5" stopColor="#6fc3a8" stopOpacity=".22" /><stop offset="1" stopColor="#8fd4bf" stopOpacity=".42" /></linearGradient>
        <linearGradient id="wf2" x1="0" y1="1" x2="1" y2="0"><stop offset="0" stopColor="#6fc3a8" stopOpacity="0" /><stop offset=".45" stopColor="#8fd4bf" stopOpacity=".55" /><stop offset="1" stopColor="#b7e6d8" stopOpacity=".95" /></linearGradient>
        <linearGradient id="wf3" x1="0" y1="1" x2="1" y2="0"><stop offset="0" stopColor="#38bdf8" stopOpacity="0" /><stop offset=".6" stopColor="#4cc3d8" stopOpacity=".35" /><stop offset="1" stopColor="#7de0e8" stopOpacity=".7" /></linearGradient>
      </defs>
      <g className="wave wave-a">
        <path d="M-6 130 C 30 124 84 116 122 92 S 190 44 218 38 V130 Z" fill="url(#wf1)" />
        <path d="M-6 130 C 30 124 84 116 122 92 S 190 44 218 38" fill="none" stroke="url(#wf2)" strokeWidth="1.1" />
      </g>
      <g className="wave wave-b">
        <path d="M24 130 C 70 124 120 116 156 98 S 204 66 218 62 V130 Z" fill="url(#wf1)" opacity=".75" />
        <path d="M24 130 C 70 124 120 116 156 98 S 204 66 218 62" fill="none" stroke="url(#wf3)" strokeWidth=".9" />
      </g>
      <g className="wave wave-c">
        <path d="M-6 130 C 50 128 110 124 150 108 S 200 84 218 80" fill="none" stroke="url(#wf2)" strokeWidth=".7" opacity=".6" />
      </g>
    </svg>
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
      className="relative mx-3 mb-3 min-h-[188px] overflow-hidden rounded-2xl bg-gradient-to-br from-[#17304a] via-[#112338] to-[#0b1829] p-4 ring-1 ring-white/10">
      <LeafOutline className="relative z-10 size-7" />
      <div key={slide.key} className="promo-in relative z-10">
        <p className="mt-3 text-[15px] font-bold leading-snug text-white">{slide.title}</p>
        <p className="mt-1.5 text-xs leading-snug text-slate-300/80">{slide.text}</p>
      </div>
      <div className="absolute right-3 top-3 z-10 flex gap-1" aria-hidden>
        {slides.map((s2, n) => <span key={s2.key} className={cn("h-1 rounded-full transition-all duration-500", n === i % slides.length ? "w-4 bg-leaf" : "w-1.5 bg-white/25")} />)}
      </div>
      <Waves />
    </div>
  );
}

export function Sidebar({ isOperator = false }: { isOperator?: boolean }) {
  const path = usePathname();
  const { navOpen, setNavOpen } = useUi();
  const t = useT();
  const items = [...NAV, ...(isOperator ? [{ href: "/operator", label: "Operator console", icon: ShieldCheck }] : [])];
  const slug = (href: string) => (href === "/operator" ? "operator" : href.slice(1).replace(/\W+/g, "_"));
  return (
    <>
      {navOpen && <button aria-label="Close menu" className="fixed inset-0 z-30 bg-slate-900/40 backdrop-blur-[2px] lg:hidden" onClick={() => setNavOpen(false)} />}
      <aside className={cn(
        "fixed inset-y-0 left-0 z-40 flex w-[236px] flex-col bg-gradient-to-b from-navy to-navy-2 text-slate-300 shadow-[4px_0_24px_rgba(8,15,30,0.18)] transition-transform duration-300 ease-[var(--ease-out-soft)] lg:translate-x-0",
        navOpen ? "translate-x-0" : "-translate-x-full")}>
        <Link href="/overview" onClick={() => setNavOpen(false)} className="flex items-center gap-3 px-5 pb-5 pt-5">
          <Logo className="h-11 w-9" />
          <span className="leading-tight"><span className="block text-[20px] font-bold tracking-tight text-white">Reliabilix</span><span className="block text-xs font-medium uppercase tracking-[0.18em] text-leaf/80">Systems</span></span>
        </Link>
        <nav aria-label="Main" className="thin-scroll flex min-h-0 flex-1 flex-col gap-1 px-2 pl-3 pt-1">
          {items.map(({ href, label, icon: Icon }) => {
            const active = path === href || path.startsWith(`${href}/`);
            return (
              <Link key={href} href={href} onClick={() => setNavOpen(false)} aria-current={active ? "page" : undefined}
                className={cn("nav-link flex items-center gap-3.5 rounded-xl px-3.5 py-3 text-[15px] font-medium", active ? "bg-navy-active text-white" : "text-slate-300")}>
                <Icon className="nav-icon size-[19px]" strokeWidth={1.8} />{t(`nav.${slug(href)}`, undefined, label)}
              </Link>
            );
          })}
        </nav>
        <PromoCard />
        <div className="px-5 pb-4 text-[11px] leading-snug text-slate-500">
          <p className="font-medium text-slate-400">Reliabilix GreenOps Platform</p>
          <p className="flex items-center gap-1.5" title={`Platform release ${BUILD_VERSION.full}`}>
            v{BUILD_VERSION.number}
            {BUILD_VERSION.stage !== "stable" && <span className="rounded bg-white/10 px-1.5 py-px text-[10px] font-semibold tracking-wide text-leaf">{BUILD_VERSION.full.split("-").slice(1).join("-") || BUILD_VERSION.stage}</span>}
          </p>
        </div>
      </aside>
    </>
  );
}
