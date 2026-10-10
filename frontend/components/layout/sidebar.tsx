"use client";

import { ShieldCheck } from "lucide-react";
import Link from "next/link";
import { usePathname } from "next/navigation";
import { content } from "@/content/schema";
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
        <linearGradient id="wf1" x1="0" y1="1" x2="1" y2="0"><stop offset="0" stopColor="#1f8f6e" stopOpacity="0" /><stop offset=".5" stopColor="#2fb985" stopOpacity=".22" /><stop offset="1" stopColor="#5be3b1" stopOpacity=".42" /></linearGradient>
        <linearGradient id="wf2" x1="0" y1="1" x2="1" y2="0"><stop offset="0" stopColor="#3ddc97" stopOpacity="0" /><stop offset=".45" stopColor="#52e0a8" stopOpacity=".55" /><stop offset="1" stopColor="#9bf5d3" stopOpacity=".95" /></linearGradient>
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

export function Sidebar({ isOperator = false }: { isOperator?: boolean }) {
  const path = usePathname();
  const { navOpen, setNavOpen } = useUi();
  const items = [...NAV, ...(isOperator ? [{ href: "/operator", label: "Operator console", icon: ShieldCheck }] : [])];
  return (
    <>
      {navOpen && <button aria-label="Close menu" className="fixed inset-0 z-30 bg-slate-900/40 backdrop-blur-[2px] lg:hidden" onClick={() => setNavOpen(false)} />}
      <aside className={cn(
        "fixed inset-y-0 left-0 z-40 flex w-[236px] flex-col bg-gradient-to-b from-navy to-navy-2 text-slate-300 shadow-[4px_0_24px_rgba(8,15,30,0.18)] transition-transform duration-300 ease-[var(--ease-out-soft)] lg:translate-x-0",
        navOpen ? "translate-x-0" : "-translate-x-full")}>
        <Link href="/overview" onClick={() => setNavOpen(false)} className="flex items-center gap-3 px-5 pb-5 pt-5">
          <Logo className="size-10" />
          <span className="leading-tight"><span className="block text-[20px] font-bold tracking-tight text-white">Reliabilix</span><span className="block text-xs text-slate-400">GreenOps Platform</span></span>
        </Link>
        <nav aria-label="Main" className="flex flex-1 flex-col gap-1 px-3 pt-1">
          {items.map(({ href, label, icon: Icon }) => {
            const active = path === href || path.startsWith(`${href}/`);
            return (
              <Link key={href} href={href} onClick={() => setNavOpen(false)} aria-current={active ? "page" : undefined}
                className={cn("nav-link flex items-center gap-3.5 rounded-xl px-3.5 py-3 text-[15px] font-medium", active ? "bg-navy-active text-white" : "text-slate-300")}>
                <Icon className="nav-icon size-[19px]" strokeWidth={1.8} />{label}
              </Link>
            );
          })}
        </nav>
        <div className="relative mx-3 mb-3 min-h-[188px] overflow-hidden rounded-2xl bg-gradient-to-br from-[#17304a] via-[#112338] to-[#0b1829] p-4 ring-1 ring-white/10">
          <LeafOutline className="relative z-10 size-7" />
          <p className="relative z-10 mt-3 text-[15px] font-bold leading-snug text-white">{content.sidebarPromo.title}</p>
          <p className="relative z-10 mt-1.5 text-xs leading-snug text-slate-300/80">{content.sidebarPromo.text}</p>
          <Waves />
        </div>
        <div className="px-5 pb-4 text-[11px] leading-snug text-slate-500">
          <p className="font-medium text-slate-400">Reliabilix GreenOps Platform</p>
          <p className="flex items-center gap-1.5" title={`Platform release ${BUILD_VERSION.full}`}>
            v{BUILD_VERSION.number}
            {BUILD_VERSION.stage !== "stable" && <span className="rounded bg-white/10 px-1.5 py-px text-[10px] font-semibold tracking-wide text-emerald-300">{BUILD_VERSION.full.split("-").slice(1).join("-") || BUILD_VERSION.stage}</span>}
          </p>
        </div>
      </aside>
    </>
  );
}
