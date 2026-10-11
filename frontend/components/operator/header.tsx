"use client";

import * as DM from "@radix-ui/react-dropdown-menu";
import { IconArrowLeft as ArrowLeft, IconLogOut as LogOut, IconMenu as Menu, IconShield as ShieldCheck } from "@/components/ui/icons";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { useMemo } from "react";
import { NotificationBell } from "@/components/layout/bell";
import { LanguageSwitcher } from "@/components/layout/language-switcher";
import { QuickSearchBox, type QuickItem } from "@/components/layout/quick-search";
import { DropdownItem, DropdownPanel, DropdownRoot } from "@/components/ui/dropdown";
import { useOperatorTenants } from "@/hooks/use-admin";
import { useT } from "@/i18n/provider";
import type { AppUser } from "@/lib/auth/session";
import { useOperatorAttention } from "./attention";
import { ALL_ITEMS, activeItem, navKey } from "./nav";

// The same pill as the client header, so the two frames read as one product.
const pill = "group flex h-10 items-center gap-2 rounded-xl border border-line/80 bg-white px-3 text-sm font-medium text-ink shadow-[0_1px_2px_rgba(16,24,40,0.04)] outline-none transition-all duration-200 hover:border-slate-300 hover:shadow-sm data-[state=open]:border-leaf data-[state=open]:shadow-[0_0_0_3px_rgba(111,195,168,0.22)] focus-visible:border-leaf";

/** Search for the console: its pages and the organizations. The same box as the client's header. */
function ConsoleSearch() {
  const t = useT();
  const router = useRouter();
  const tenants = useOperatorTenants();
  const all = useMemo<QuickItem[]>(() => [
    ...ALL_ITEMS.map((n) => ({ label: t(navKey(n.href), undefined, n.label), hint: t("search.page"), run: () => router.push(n.href) })),
    ...(tenants.data ?? []).map((o) => ({ label: o.name, hint: t("operator.search.organization"), run: () => router.push(`/operator/organizations/${o.id}`) })),
  ], [t, router, tenants.data]);
  return <QuickSearchBox all={all} />;
}

/**
 * The console header is the client's: the page it is on (title and its purpose), then search, language, the bell and the
 * person. The bell is the client's bell, with what needs attention on the platform added to it for an operator.
 */
export function OperatorHeader({ user, role, path, onMenu }: { user: AppUser; role: string; path: string; onMenu: () => void }) {
  const t = useT();
  const attention = useOperatorAttention();
  const current = activeItem(path);
  const initials = user.name.split(/\s+/).map((w) => w[0]).slice(0, 2).join("").toUpperCase();
  const extra = useMemo(() => attention.items.map((a) => ({ key: a.key, title: a.text, href: a.href })), [attention.items]);

  return (
    <header className="sticky top-0 z-30 flex h-[68px] items-center gap-3 border-b border-line/70 bg-white/85 px-4 backdrop-blur-xl lg:px-6">
      <button className="rounded-xl p-2 hover:bg-slate-100 lg:hidden" aria-label={t("header.menu")} onClick={onMenu}><Menu className="size-5" /></button>
      <div key={current?.href ?? "console"} className="rise flex min-w-0 items-center gap-3">
        <span className="grid size-10 shrink-0 place-items-center rounded-xl bg-leaf-soft/70 text-leaf-deep ring-1 ring-leaf/20">
          {current ? <current.icon className="size-5" strokeWidth={1.8} aria-hidden /> : <ShieldCheck className="size-5" aria-hidden />}
        </span>
        <div className="min-w-0 leading-tight">
          <p className="truncate text-[15px] font-semibold tracking-tight">{current ? t(navKey(current.href), undefined, current.label) : t("nav.operator")}</p>
          <p className="hidden truncate text-xs text-muted sm:block">{t("operator.header.subtitle")}</p>
        </div>
      </div>

      <div className="ml-auto flex items-center gap-2.5">
        <ConsoleSearch />
        <LanguageSwitcher className={pill} />
        <NotificationBell extra={extra} extraReady={!attention.loading} allHref="/operator" settingsHref="/operator/settings" />
        <DropdownRoot>
          <DM.Trigger className="group flex items-center gap-2.5 rounded-xl py-1 pl-1 pr-3 outline-none transition-colors hover:bg-slate-100 data-[state=open]:bg-slate-100">
            <span className="flex size-9 items-center justify-center rounded-lg bg-leaf-deep text-sm font-bold text-white">{initials || "U"}</span>
            <span className="hidden text-left leading-tight sm:block"><span className="block text-sm font-semibold">{user.name}</span><span className="block text-xs capitalize text-muted">{role === "admin" ? t("operator.role.operator") : role}</span></span>
          </DM.Trigger>
          <DropdownPanel align="end">
            <DropdownItem asChild><Link href="/overview"><ArrowLeft className="size-4" />{t("nav.back_to_app")}</Link></DropdownItem>
            <DropdownItem asChild><a href="/auth/logout"><LogOut className="size-4" />{t("header.sign_out")}</a></DropdownItem>
          </DropdownPanel>
        </DropdownRoot>
      </div>
    </header>
  );
}
