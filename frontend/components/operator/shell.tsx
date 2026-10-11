"use client";

import { IconArrowLeft as ArrowLeft, IconLogOut as LogOut, IconMenu as Menu, IconSearch as Search, IconShield as ShieldCheck } from "@/components/ui/icons";
import Link from "next/link";
import { usePathname } from "next/navigation";
import { useState, type CSSProperties, type ReactNode } from "react";
import { CollapseToggle } from "@/components/layout/collapse-toggle";
import { Logo } from "@/components/layout/logo";
import { Badge } from "@/components/ui/badge";
import { DropdownItem, DropdownPanel, DropdownRoot, DropdownTrigger } from "@/components/ui/dropdown";
import { LanguageSwitcher } from "@/components/layout/language-switcher";
import { useT } from "@/i18n/provider";
import { useUi } from "@/lib/store";
import { cn } from "@/lib/utils";
import { BUILD_VERSION } from "@/lib/version";
import { activeItem, navKey, OPERATOR_NAV } from "./nav";
import { PageSearch } from "./palette";

/** The console uses the same frame as the client workspace: dark left panel, white header, white content. */
export function OperatorShell({ operator, children }: { operator: { email: string; subject: string; role: string }; children: ReactNode }) {
  const path = usePathname();
  const { navOpen, setNavOpen, navCollapsed } = useUi();
  const slim = navCollapsed;
  const [search, setSearch] = useState(false);
  const t = useT();
  const current = activeItem(path);
  const who = operator.email || operator.subject;
  const initials = (who.match(/[A-Za-z0-9]/g) ?? ["O"]).slice(0, 2).join("").toUpperCase();

  return (
    <div className="sb-shell min-h-screen" style={{ "--sb": slim ? "76px" : "236px" } as CSSProperties}>
      {navOpen && <button aria-label="Close menu" className="fixed inset-0 z-30 bg-slate-900/40 backdrop-blur-[2px] lg:hidden" onClick={() => setNavOpen(false)} />}
      <aside className={cn(
        "sb-aside fixed inset-y-0 left-0 z-40 flex w-[236px] flex-col overflow-hidden bg-gradient-to-b from-navy to-navy-2 text-slate-300 shadow-[4px_0_24px_rgba(8,15,30,0.18)] lg:translate-x-0",
        navOpen ? "translate-x-0" : "-translate-x-full")}>
        <div className={cn("flex items-center pb-4 pt-5", slim ? "flex-col gap-3 px-0" : "justify-between pl-5 pr-3")}>
          <Link href="/operator" onClick={() => setNavOpen(false)} className="flex items-center gap-3" title="Reliabilix">
            <Logo className="size-10 shrink-0" />
            {!slim && <span className="leading-tight"><span className="block text-[20px] font-bold tracking-tight text-white">Reliabilix</span><span className="block text-xs text-slate-400">{t("nav.operator")}</span></span>}
          </Link>
          <CollapseToggle className="hidden lg:grid" />
        </div>
        <nav aria-label="Operator" className={cn("thin-scroll flex min-h-0 flex-1 flex-col pb-3 pt-1", slim ? "items-center gap-2 px-2" : "gap-4 px-2 pl-3")}>
          {OPERATOR_NAV.map((g) => (
            <div key={g.title}>
              <p className={cn("mb-1 px-3.5 text-[11px] font-semibold uppercase tracking-wider text-slate-500", slim && "hidden")}>{t(`operator.group.${g.title.toLowerCase()}`, undefined, g.title)}</p>
              <ul className={cn("space-y-0.5", slim && "flex flex-col items-center")}>
                {g.items.map(({ href, label, icon: Icon, planned }) => {
                  const active = current?.href === href;
                  return (
                    <li key={href}>
                      <Link href={href} onClick={() => setNavOpen(false)} aria-current={active ? "page" : undefined}
                        title={slim ? t(navKey(href), undefined, label) : undefined} aria-label={slim ? t(navKey(href), undefined, label) : undefined}
                        className={cn("nav-link flex items-center rounded-xl text-[14.5px] font-medium", slim ? "size-11 justify-center" : "gap-3.5 px-3.5 py-2.5", active ? "bg-navy-active text-white" : "text-slate-300")}>
                        <Icon className={cn("nav-icon shrink-0", slim ? "size-[21px]" : "size-[18px]")} strokeWidth={1.8} />
                        {!slim && <span className="truncate">{t(navKey(href), undefined, label)}</span>}
                        {planned && !slim && <span className="ml-auto rounded bg-white/10 px-1.5 py-px text-[10px] font-semibold text-violet-300">{t("operator.planned")}</span>}
                      </Link>
                    </li>
                  );
                })}
              </ul>
            </div>
          ))}
        </nav>
        <Link href="/overview" onClick={() => setNavOpen(false)} title={slim ? t("nav.back_to_app") : undefined}
          className={cn("nav-link mb-2 flex items-center rounded-xl text-[14.5px] font-medium text-slate-300", slim ? "mx-auto size-11 justify-center" : "mx-3 gap-3.5 px-3.5 py-2.5")}>
          <ArrowLeft className="nav-icon size-[18px]" strokeWidth={1.8} />{!slim && t("nav.back_to_app")}
        </Link>
        <div className={cn("px-5 pb-4 text-[11px] leading-snug text-slate-500", slim && "hidden")}>
          <p className="font-medium text-slate-400">Reliabilix GreenOps Platform</p>
          <p className="flex items-center gap-1.5" title={`Platform release ${BUILD_VERSION.full}`}>
            v{BUILD_VERSION.number}
            {BUILD_VERSION.stage !== "stable" && <span className="rounded bg-white/10 px-1.5 py-px text-[10px] font-semibold tracking-wide text-leaf">{BUILD_VERSION.full.split("-").slice(1).join("-") || BUILD_VERSION.stage}</span>}
          </p>
        </div>
      </aside>

      <header className="sticky top-0 z-30 flex h-16 items-center gap-3 border-b border-line bg-white/90 px-4 backdrop-blur-xl lg:px-6">
        <button className="rounded-lg p-2 hover:bg-slate-100 lg:hidden" aria-label="Open menu" onClick={() => setNavOpen(true)}><Menu className="size-5" /></button>
        <ShieldCheck className="size-5 text-primary" aria-hidden />
        <div className="leading-tight">
          <p className="text-sm font-semibold">{current ? t(navKey(current.href), undefined, current.label) : t("nav.operator")}</p>
          <p className="hidden text-xs text-muted sm:block">Platform administration</p>
        </div>
        <div className="ml-auto flex items-center gap-3">
          <LanguageSwitcher />
          <button type="button" onClick={() => setSearch(true)} aria-label="Go to page"
            className="relative hidden w-64 rounded-xl bg-white py-2.5 pl-10 pr-3 text-left text-sm text-slate-400 shadow-[0_1px_2px_rgba(16,24,40,0.04)] ring-1 ring-line transition hover:ring-slate-300 md:block lg:w-72">
            <Search className="absolute left-3.5 top-1/2 size-4 -translate-y-1/2" aria-hidden />Search pages…
          </button>
          <button type="button" onClick={() => setSearch(true)} aria-label="Go to page" className="rounded-xl p-2.5 ring-1 ring-line md:hidden"><Search className="size-4" /></button>
          <Badge tone={operator.role === "admin" ? "good" : "neutral"}>{operator.role}</Badge>
          <DropdownRoot>
            <DropdownTrigger className="!border-0 !p-1.5 !shadow-none !ring-0 hover:!bg-slate-50">
              <span className="flex size-10 items-center justify-center rounded-full bg-gradient-to-br from-[#2b3a67] to-[#4a63c9] text-sm font-bold text-white shadow-md shadow-indigo-900/20">{initials}</span>
              <span className="hidden max-w-44 text-left leading-tight sm:block"><span className="block truncate text-sm font-semibold">{who}</span><span className="block text-xs capitalize text-muted">Operator</span></span>
            </DropdownTrigger>
            <DropdownPanel align="end">
              <DropdownItem asChild><a href="/auth/logout"><LogOut className="size-4" />Sign out</a></DropdownItem>
            </DropdownPanel>
          </DropdownRoot>
        </div>
      </header>

      <main id="op-main" className="page-enter min-w-0 overflow-x-clip p-4 lg:p-6" key={path}>{children}</main>
      <PageSearch open={search} onClose={() => setSearch(false)} />
    </div>
  );
}
