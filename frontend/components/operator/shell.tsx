"use client";

import { IconArrowLeft as ArrowLeft } from "@/components/ui/icons";
import Link from "next/link";
import { usePathname } from "next/navigation";
import type { CSSProperties, ReactNode } from "react";
import { CollapseToggle } from "@/components/layout/collapse-toggle";
import { Logo } from "@/components/layout/logo";
import { useT } from "@/i18n/provider";
import { useUi } from "@/lib/store";
import { cn } from "@/lib/utils";
import { BUILD_VERSION } from "@/lib/version";
import { activeItem, navKey, OPERATOR_NAV } from "./nav";
import type { AppUser } from "@/lib/auth/session";
import { OperatorHeader } from "./header";

/** The console uses the same frame as the client workspace: dark left panel, white header, white content. */
export function OperatorShell({ operator, user, children }: { operator: { email: string; subject: string; role: string }; user: AppUser; children: ReactNode }) {
  const path = usePathname();
  const { navOpen, setNavOpen, navCollapsed } = useUi();
  const slim = navCollapsed;
  const t = useT();
  const current = activeItem(path);

  return (
    <div className="sb-shell min-h-screen" style={{ "--sb": slim ? "76px" : "236px" } as CSSProperties}>
      {navOpen && <button aria-label="Close menu" className="fixed inset-0 z-30 bg-slate-900/40 backdrop-blur-[2px] lg:hidden" onClick={() => setNavOpen(false)} />}
      <aside className={cn(
        "sb-aside fixed inset-y-0 left-0 z-40 flex w-[236px] flex-col overflow-hidden bg-gradient-to-b from-navy to-navy-2 text-slate-300 shadow-[4px_0_24px_rgba(8,15,30,0.18)] lg:translate-x-0",
        navOpen ? "translate-x-0" : "-translate-x-full")}>
        <div className={cn("flex items-center pb-3 pt-4", slim ? "flex-col gap-3 px-0" : "justify-between pl-5 pr-3")}>
          <Link href="/operator" onClick={() => setNavOpen(false)} className="flex items-center gap-3" title="Reliabilix">
            <Logo className="size-10 shrink-0" />
            {!slim && <span className="leading-tight"><span className="block text-[20px] font-bold tracking-tight text-white">Reliabilix</span><span className="block text-xs text-slate-400">{t("nav.operator")}</span></span>}
          </Link>
          <CollapseToggle className="hidden lg:grid" />
        </div>
        <nav aria-label="Operator" className={cn("thin-scroll flex min-h-0 flex-1 flex-col pb-6 pt-1 [mask-image:linear-gradient(to_bottom,#000_calc(100%-22px),transparent)]", slim ? "nav-slim items-center gap-2 px-2" : "gap-2 px-2 pl-3")}>
          {OPERATOR_NAV.map((g) => (
            <div key={g.title}>
              <p className={cn("mb-0.5 px-3.5 text-[11px] font-semibold uppercase tracking-wider text-slate-500", slim && "hidden")}>{t(`operator.group.${g.title.toLowerCase()}`, undefined, g.title)}</p>
              <ul className={cn("space-y-0.5", slim && "flex flex-col items-center")}>
                {g.items.map(({ href, label, icon: Icon, planned }) => {
                  const active = current?.href === href;
                  return (
                    <li key={href}>
                      <Link href={href} onClick={() => setNavOpen(false)} aria-current={active ? "page" : undefined}
                        title={slim ? t(navKey(href), undefined, label) : undefined} aria-label={slim ? t(navKey(href), undefined, label) : undefined}
                        className={cn("nav-link flex items-center rounded-xl text-[14.5px] font-medium", slim ? "size-11 justify-center" : "gap-3.5 px-3.5 py-2", active ? "bg-navy-active text-white" : "text-slate-300")}>
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
        <div className={cn("border-t border-white/5 pb-3 pt-2", slim ? "flex flex-col items-center gap-1" : "px-3")}>
          <Link href="/overview" onClick={() => setNavOpen(false)} title={slim ? t("nav.back_to_app") : undefined} aria-label={t("nav.back_to_app")}
            className={cn("nav-link flex items-center rounded-xl text-[14px] font-medium text-slate-300", slim ? "size-11 justify-center" : "gap-3.5 px-3.5 py-[6px]")}>
            <ArrowLeft className="nav-icon size-[18px]" strokeWidth={1.8} />{!slim && t("nav.back_to_app")}
          </Link>
          {!slim && (
            <p className="flex items-center gap-1.5 px-3.5 pt-1 text-[11px] text-slate-500" title={`Platform release ${BUILD_VERSION.full}`}>
              v{BUILD_VERSION.number}
              {BUILD_VERSION.stage !== "stable" && <span className="rounded bg-white/10 px-1.5 py-px text-[10px] font-semibold tracking-wide text-leaf">{BUILD_VERSION.full.split("-").slice(1).join("-") || BUILD_VERSION.stage}</span>}
            </p>
          )}
        </div>
      </aside>

      <OperatorHeader user={user} role={operator.role} path={path} onMenu={() => setNavOpen(true)} />

      <main id="op-main" className="page-enter min-w-0 overflow-x-clip p-4 lg:p-6" key={path}>{children}</main>
    </div>
  );
}
