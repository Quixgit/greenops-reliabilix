"use client";

import { IconLogOut as LogOut, IconMenu as Menu, IconFolder, IconSliders } from "@/components/ui/icons";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { useMemo, useState } from "react";
import { Calendar } from "@/components/ui/calendar";
import { DropdownItem, DropdownPanel, DropdownRadioGroup, DropdownRadioItem, DropdownRoot } from "@/components/ui/dropdown";
import * as DM from "@radix-ui/react-dropdown-menu";
import { LanguageSwitcher } from "@/components/layout/language-switcher";
import { NAV } from "@/components/layout/nav";
import { useConnections, useProjects } from "@/hooks/use-overview";
import { NotificationBell } from "@/components/layout/bell";
import { QuickSearchBox, type QuickItem } from "@/components/layout/quick-search";
import { useT } from "@/i18n/provider";

import type { Role } from "@/lib/permissions";
import { PERIODS, periodLabel, useUi } from "@/lib/store";
import { cn } from "@/lib/utils";

const ALL = "__all__";
const pill = "group flex h-10 items-center gap-2 rounded-xl border border-line/80 bg-white px-3 text-sm font-medium text-ink shadow-[0_1px_2px_rgba(16,24,40,0.04)] outline-none transition-all duration-200 hover:border-slate-300 hover:shadow-sm data-[state=open]:border-leaf data-[state=open]:shadow-[0_0_0_3px_rgba(111,195,168,0.22)] focus-visible:border-leaf";

/** Jump to a page, a project or a connection by typing. ⌘K / Ctrl+K focuses it; Enter opens the first match. */
function QuickSearch() {
  const t = useT();
  const router = useRouter();
  const projects = useProjects();
  const conns = useConnections();
  const setProject = useUi((s) => s.setProject);
  const all = useMemo<QuickItem[]>(() => [
    ...NAV.map((n) => ({ label: n.label, hint: t("search.page"), run: () => router.push(n.href) })),
    ...(projects.data ?? []).map((p) => ({ label: p.name, hint: t("search.project"), run: () => { setProject(p.id); router.push("/overview"); } })),
    ...(conns.data ?? []).map((c) => ({ label: `${c.provider.toUpperCase()} ${c.account_ref}`, hint: t("search.connection"), run: () => router.push(`/cloud/${c.id}`) })),
  ], [t, router, projects.data, conns.data, setProject]);
  return <QuickSearchBox all={all} />;
}

/** The period: three presets as one segmented control and a calendar for any other range. */
function PeriodPicker() {
  const t = useT();
  const { period, setPeriod, custom, setCustom } = useUi();
  const [draft, setDraft] = useState<{ from?: string; to?: string }>({});
  const [open, setOpen] = useState(false);
  return (
    <div className="order-last flex w-full items-center gap-2 lg:order-none lg:w-auto">
      <div role="tablist" aria-label={t("header.period")} className="flex h-10 items-center rounded-xl border border-line/80 bg-white p-1 shadow-[0_1px_2px_rgba(16,24,40,0.04)]">
        {PERIODS.map((p) => (
          <button key={p.key} role="tab" type="button" aria-selected={period === p.key} onClick={() => setPeriod(p.key)}
            className={cn("rounded-lg px-3 py-1 text-[13px] font-medium transition-all duration-200", period === p.key ? "bg-leaf-soft text-leaf-deep shadow-sm" : "text-slate-500 hover:text-ink")}>
            {p.days}d
          </button>
        ))}
      </div>
      <DropdownRoot open={open} onOpenChange={(o) => { setOpen(o); if (o) setDraft(custom ?? {}); }}>
        <DM.Trigger className={cn(pill, period === "custom" && "border-leaf/60 bg-leaf-soft/50")}>
          <span className="text-slate-500"><IconCalendar /></span>
          <span className="max-w-44 truncate">{period === "custom" ? periodLabel(period, custom) : t("header.custom_range")}</span>
        </DM.Trigger>
        <DropdownPanel align="end">
          <div className="p-2">
            <Calendar mode="range" from={draft.from} to={draft.to} max={new Date().toISOString().slice(0, 10)}
              onPick={(f, to) => setDraft({ from: f, to })} />
            <div className="mt-2 flex items-center justify-between gap-2 border-t border-line/70 px-1 pt-2.5">
              <span className="text-xs text-muted">{draft.from ? `${draft.from}${draft.to ? ` → ${draft.to}` : ""}` : t("header.pick_range")}</span>
              <button type="button" disabled={!draft.from} onClick={() => { setCustom(draft.from!, draft.to ?? draft.from!); setOpen(false); }}
                className="rounded-xl btn-leaf px-3 py-1.5 text-xs font-semibold text-white transition disabled:opacity-40">{t("header.apply")}</button>
            </div>
          </div>
        </DropdownPanel>
      </DropdownRoot>
    </div>
  );
}

function IconCalendar() {
  return (
    <svg viewBox="0 0 24 24" className="size-[18px]" fill="none" stroke="currentColor" strokeWidth="1.6" strokeLinecap="round" strokeLinejoin="round" aria-hidden>
      <path d="M5.5 5.6h13c.9 0 1.6.7 1.6 1.6v10.6c0 .9-.7 1.6-1.6 1.6h-13c-.9 0-1.6-.7-1.6-1.6V7.2c0-.9.7-1.6 1.6-1.6z" fill="currentColor" fillOpacity=".12" stroke="none" />
      <path d="M5.5 5.6h13c.9 0 1.6.7 1.6 1.6v10.6c0 .9-.7 1.6-1.6 1.6h-13c-.9 0-1.6-.7-1.6-1.6V7.2c0-.9.7-1.6 1.6-1.6z" /><path d="M3.9 10h16.2M8.2 3.6v3.2M15.8 3.6v3.2" />
    </svg>
  );
}

export function Header({ user }: { user: { name: string; email: string; role: Role }; unread?: number }) {
  const { projectId, setProject, setNavOpen } = useUi();
  const t = useT();
  const projects = useProjects();
  const selected = projects.data?.find((p) => p.id === projectId);
  const initials = user.name.split(/\s+/).map((w) => w[0]).slice(0, 2).join("").toUpperCase();

  return (
    <header className="sticky top-0 z-30 flex min-h-[68px] flex-wrap items-center gap-x-2 gap-y-2 border-b border-line/70 bg-white/85 px-4 py-2 backdrop-blur-xl sm:gap-x-3 lg:flex-nowrap lg:px-6 lg:py-0">
      <button className="rounded-xl p-2 hover:bg-slate-100 lg:hidden" aria-label={t("header.menu")} onClick={() => setNavOpen(true)}><Menu className="size-5" /></button>

      <DropdownRoot>
        <DM.Trigger className={pill} aria-label={selected?.name ?? t("header.all_projects")}>
          <IconFolder className="size-6 text-leaf-deep" />
          <span className="max-w-[5.25rem] truncate max-[430px]:hidden sm:max-w-44">{selected?.name ?? t("header.all_projects")}</span>
          <svg viewBox="0 0 12 12" className="size-3 text-slate-400 transition-transform duration-300 group-data-[state=open]:rotate-180" fill="none" stroke="currentColor" strokeWidth="1.6" strokeLinecap="round"><path d="M2.5 4.5 6 8l3.5-3.5" /></svg>
        </DM.Trigger>
        <DropdownPanel>
          <DropdownRadioGroup value={projectId ?? ALL} onValueChange={(v) => setProject(v === ALL ? null : v)}>
            <DropdownRadioItem value={ALL}>{t("header.all_projects")}</DropdownRadioItem>
            {projects.data?.map((p) => <DropdownRadioItem key={p.id} value={p.id}>{p.name}</DropdownRadioItem>)}
          </DropdownRadioGroup>
        </DropdownPanel>
      </DropdownRoot>

      <PeriodPicker />

      <div className="ml-auto flex items-center gap-2 sm:gap-2.5">
        <QuickSearch />
        <LanguageSwitcher className={cn(pill, "max-sm:gap-0 max-sm:px-2.5 max-sm:[&>span.uppercase]:hidden max-sm:[&>svg:last-child]:hidden")} />

        <NotificationBell />

        <DropdownRoot>
          <DM.Trigger className="group flex items-center gap-2.5 rounded-xl py-1 pl-1 pr-3 outline-none transition-colors hover:bg-slate-100 data-[state=open]:bg-slate-100">
            <span className="flex size-9 items-center justify-center rounded-lg bg-leaf-deep text-sm font-bold text-white">{initials || "U"}</span>
            <span className="hidden text-left leading-tight sm:block"><span className="block text-sm font-semibold">{user.name}</span><span className="block text-xs capitalize text-muted">{user.role}</span></span>
          </DM.Trigger>
          <DropdownPanel align="end">
            <DropdownItem asChild><Link href="/settings"><IconSliders className="size-4" />{t("header.settings")}</Link></DropdownItem>
            <DropdownItem asChild><a href="/auth/logout"><LogOut className="size-4" />{t("header.sign_out")}</a></DropdownItem>
          </DropdownPanel>
        </DropdownRoot>
      </div>
    </header>
  );
}
