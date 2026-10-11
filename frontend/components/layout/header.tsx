"use client";

import { IconLogOut as LogOut, IconMenu as Menu, IconBell, IconFolder, IconSearch, IconSliders } from "@/components/ui/icons";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { useEffect, useMemo, useRef, useState } from "react";
import { Calendar } from "@/components/ui/calendar";
import { DropdownItem, DropdownPanel, DropdownRadioGroup, DropdownRadioItem, DropdownRoot } from "@/components/ui/dropdown";
import * as DM from "@radix-ui/react-dropdown-menu";
import { LanguageSwitcher } from "@/components/layout/language-switcher";
import { NAV } from "@/components/layout/nav";
import { useConnections, useProjects } from "@/hooks/use-overview";
import { NotificationBell } from "@/components/layout/bell";
import { useT } from "@/i18n/provider";
import { timeAgo } from "@/lib/formatters";
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
  const [q, setQ] = useState("");
  const [open, setOpen] = useState(false);
  const [cur, setCur] = useState(0);
  const box = useRef<HTMLInputElement>(null);
  useEffect(() => {
    const k = (e: KeyboardEvent) => { if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === "k") { e.preventDefault(); box.current?.focus(); setOpen(true); } };
    window.addEventListener("keydown", k);
    return () => window.removeEventListener("keydown", k);
  }, []);
  const items = useMemo(() => {
    const all: { label: string; hint: string; run: () => void }[] = [
      ...NAV.map((n) => ({ label: n.label, hint: t("search.page"), run: () => router.push(n.href) })),
      ...(projects.data ?? []).map((p) => ({ label: p.name, hint: t("search.project"), run: () => { setProject(p.id); router.push("/overview"); } })),
      ...(conns.data ?? []).map((c) => ({ label: `${c.provider.toUpperCase()} ${c.account_ref}`, hint: t("search.connection"), run: () => router.push(`/cloud/${c.id}`) })),
    ];
    const n = q.trim().toLowerCase();
    return (n ? all.filter((i) => i.label.toLowerCase().includes(n)) : all.slice(0, 7)).slice(0, 8);
  }, [q, t, router, projects.data, conns.data, setProject]);
  const go = (i: number) => { items[i]?.run(); setOpen(false); setQ(""); box.current?.blur(); };
  return (
    <div className="relative hidden md:block">
      <IconSearch className="pointer-events-none absolute left-3.5 top-1/2 size-[18px] -translate-y-1/2 text-slate-400" />
      <input ref={box} value={q} placeholder={t("header.search")} aria-label={t("header.search")} role="combobox" aria-expanded={open} aria-controls="quick-search"
        onChange={(e) => { setQ(e.target.value); setCur(0); setOpen(true); }} onFocus={() => setOpen(true)} onBlur={() => window.setTimeout(() => setOpen(false), 120)}
        onKeyDown={(e) => {
          if (e.key === "ArrowDown") { e.preventDefault(); setCur((c) => Math.min(items.length - 1, c + 1)); }
          else if (e.key === "ArrowUp") { e.preventDefault(); setCur((c) => Math.max(0, c - 1)); }
          else if (e.key === "Enter") { e.preventDefault(); go(cur); }
          else if (e.key === "Escape") { setOpen(false); box.current?.blur(); }
        }}
        className="h-10 w-60 rounded-xl border border-line/80 bg-white pl-11 pr-14 text-sm text-ink shadow-[0_1px_2px_rgba(16,24,40,0.04)] placeholder:text-slate-400 lg:w-72" />
      <kbd className="pointer-events-none absolute right-3 top-1/2 -translate-y-1/2 rounded-md bg-slate-100 px-1.5 py-0.5 text-[10px] font-medium text-slate-500">⌘K</kbd>
      {open && items.length > 0 && (
        <ul id="quick-search" role="listbox" className="info-pop absolute right-0 top-12 z-50 w-80 overflow-hidden rounded-2xl border border-line bg-white p-1.5 shadow-[0_18px_44px_rgba(31,52,110,0.16)]">
          {items.map((it, i) => (
            <li key={`${it.hint}-${it.label}`} role="option" aria-selected={i === cur}>
              <button type="button" onMouseDown={(e) => e.preventDefault()} onClick={() => go(i)} onMouseEnter={() => setCur(i)}
                className={cn("flex w-full items-center justify-between gap-3 rounded-lg px-3 py-2 text-left text-sm transition-colors", i === cur ? "bg-leaf-soft/70" : "")}>
                <span className="truncate font-medium">{it.label}</span><span className="shrink-0 text-xs text-muted">{it.hint}</span>
              </button>
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}

/** The period: three presets as one segmented control and a calendar for any other range. */
function PeriodPicker() {
  const t = useT();
  const { period, setPeriod, custom, setCustom } = useUi();
  const [draft, setDraft] = useState<{ from?: string; to?: string }>({});
  const [open, setOpen] = useState(false);
  return (
    <div className="hidden items-center gap-2 sm:flex">
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
                className="rounded-lg btn-leaf px-3 py-1.5 text-xs font-semibold text-white transition disabled:opacity-40">{t("header.apply")}</button>
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

export function Header({ user, unread = 0 }: { user: { name: string; email: string; role: Role }; unread?: number }) {
  const { projectId, setProject, setNavOpen } = useUi();
  const t = useT();
  const projects = useProjects();
  const selected = projects.data?.find((p) => p.id === projectId);
  const initials = user.name.split(/\s+/).map((w) => w[0]).slice(0, 2).join("").toUpperCase();

  return (
    <header className="sticky top-0 z-30 flex h-[68px] items-center gap-3 border-b border-line/70 bg-white/85 px-4 backdrop-blur-xl lg:px-6">
      <button className="rounded-xl p-2 hover:bg-slate-100 lg:hidden" aria-label={t("header.menu")} onClick={() => setNavOpen(true)}><Menu className="size-5" /></button>

      <DropdownRoot>
        <DM.Trigger className={pill}>
          <IconFolder className="size-6 text-leaf-deep" />
          <span className="max-w-44 truncate">{selected?.name ?? t("header.all_projects")}</span>
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

      <div className="ml-auto flex items-center gap-2.5">
        <QuickSearch />
        <LanguageSwitcher className={pill} />

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
