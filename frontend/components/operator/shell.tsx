"use client";

import { ArrowLeft, LogOut, Menu, Search } from "lucide-react";
import { IconShield as ShieldCheck } from "@/components/ui/icons";
import Link from "next/link";
import { usePathname } from "next/navigation";
import { useState, type ReactNode } from "react";
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
  const { navOpen, setNavOpen } = useUi();
  const [search, setSearch] = useState(false);
  const t = useT();
  const current = activeItem(path);
  const who = operator.email || operator.subject;
  const initials = (who.match(/[A-Za-z0-9]/g) ?? ["O"]).slice(0, 2).join("").toUpperCase();

  return (
    <div className="min-h-screen lg:pl-[236px]">
      {navOpen && <button aria-label="Close menu" className="fixed inset-0 z-30 bg-slate-900/40 backdrop-blur-[2px] lg:hidden" onClick={() => setNavOpen(false)} />}
      <aside className={cn(
        "fixed inset-y-0 left-0 z-40 flex w-[236px] flex-col bg-gradient-to-b from-navy to-navy-2 text-slate-300 shadow-[4px_0_24px_rgba(8,15,30,0.18)] transition-transform duration-300 ease-[var(--ease-out-soft)] lg:translate-x-0",
        navOpen ? "translate-x-0" : "-translate-x-full")}>
        <Link href="/operator" onClick={() => setNavOpen(false)} className="flex items-center gap-3 px-5 pb-4 pt-5">
          <Logo className="size-10" />
          <span className="leading-tight"><span className="block text-[20px] font-bold tracking-tight text-white">Reliabilix</span><span className="block text-xs text-slate-400">{t("nav.operator")}</span></span>
        </Link>
        <nav aria-label="Operator" className="thin-scroll flex min-h-0 flex-1 flex-col gap-4 px-2 pb-3 pl-3 pt-1">
          {OPERATOR_NAV.map((g) => (
            <div key={g.title}>
              <p className="mb-1 px-3.5 text-[11px] font-semibold uppercase tracking-wider text-slate-500">{t(`operator.group.${g.title.toLowerCase()}`, undefined, g.title)}</p>
              <ul className="space-y-0.5">
                {g.items.map(({ href, label, icon: Icon, planned }) => {
                  const active = current?.href === href;
                  return (
                    <li key={href}>
                      <Link href={href} onClick={() => setNavOpen(false)} aria-current={active ? "page" : undefined}
                        className={cn("nav-link flex items-center gap-3.5 rounded-xl px-3.5 py-2.5 text-[14.5px] font-medium", active ? "bg-navy-active text-white" : "text-slate-300")}>
                        <Icon className="nav-icon size-[18px] shrink-0" strokeWidth={1.8} />
                        <span className="truncate">{t(navKey(href), undefined, label)}</span>
                        {planned && <span className="ml-auto rounded bg-white/10 px-1.5 py-px text-[10px] font-semibold text-violet-300">{t("operator.planned")}</span>}
                      </Link>
                    </li>
                  );
                })}
              </ul>
            </div>
          ))}
        </nav>
        <Link href="/overview" onClick={() => setNavOpen(false)} className="nav-link mx-3 mb-2 flex items-center gap-3.5 rounded-xl px-3.5 py-2.5 text-[14.5px] font-medium text-slate-300">
          <ArrowLeft className="nav-icon size-[18px]" strokeWidth={1.8} />{t("nav.back_to_app")}
        </Link>
        <div className="px-5 pb-4 text-[11px] leading-snug text-slate-500">
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
