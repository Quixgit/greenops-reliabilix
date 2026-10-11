"use client";

import { CalendarDays, LogOut, Menu, Search } from "lucide-react";
import { IconBell as Bell, IconDatabase as Layers, IconSliders as SettingsIcon } from "@/components/ui/icons";
import Link from "next/link";
import { DropdownItem, DropdownPanel, DropdownRadioGroup, DropdownRadioItem, DropdownRoot, DropdownTrigger } from "@/components/ui/dropdown";
import { useProjects } from "@/hooks/use-overview";
import { LanguageSwitcher } from "@/components/layout/language-switcher";
import { useT } from "@/i18n/provider";
import { PERIODS, useUi, type PeriodKey } from "@/lib/store";
import type { Role } from "@/lib/permissions";

const ALL = "__all__";

export function Header({ user, unread = 0 }: { user: { name: string; email: string; role: Role }; unread?: number }) {
  const { projectId, setProject, period, setPeriod, setNavOpen } = useUi();
  const t = useT();
  const projects = useProjects();
  const selected = projects.data?.find((p) => p.id === projectId);
  const initials = user.name.split(/\s+/).map((w) => w[0]).slice(0, 2).join("").toUpperCase();

  return (
    <header className="sticky top-0 z-30 flex h-16 items-center gap-3 border-b border-line bg-white/90 px-4 backdrop-blur-xl lg:px-6">
      <button className="rounded-lg p-2 hover:bg-slate-100 lg:hidden" aria-label={t("header.menu")} onClick={() => setNavOpen(true)}><Menu className="size-5" /></button>

      <div className="flex items-center gap-2">
        <DropdownRoot>
          <DropdownTrigger><Layers className="size-4 text-slate-500" /><span className="max-w-40 truncate">{selected?.name ?? t("header.all_projects")}</span></DropdownTrigger>
          <DropdownPanel>
            <DropdownRadioGroup value={projectId ?? ALL} onValueChange={(v) => setProject(v === ALL ? null : v)}>
              <DropdownRadioItem value={ALL}>{t("header.all_projects")}</DropdownRadioItem>
              {projects.data?.map((p) => <DropdownRadioItem key={p.id} value={p.id}>{p.name}</DropdownRadioItem>)}
            </DropdownRadioGroup>
          </DropdownPanel>
        </DropdownRoot>
        <DropdownRoot>
          <DropdownTrigger className="hidden sm:flex"><CalendarDays className="size-4 text-slate-500" />{PERIODS.find((p) => p.key === period)!.label}</DropdownTrigger>
          <DropdownPanel>
            <DropdownRadioGroup value={period} onValueChange={(v) => setPeriod(v as PeriodKey)}>
              {PERIODS.map((p) => <DropdownRadioItem key={p.key} value={p.key}>{p.label}</DropdownRadioItem>)}
            </DropdownRadioGroup>
          </DropdownPanel>
        </DropdownRoot>
      </div>

      <div className="ml-auto flex items-center gap-3">
        <LanguageSwitcher />
        {/* Stub: global search arrives in a later release. It looks like the final field but is read-only. */}
        <label className="group relative hidden md:block">
          <Search className="absolute left-3.5 top-1/2 size-4 -translate-y-1/2 text-slate-400" />
          <input readOnly aria-label="Search (coming soon)" title="Search is coming soon" placeholder="Search…"
            className="w-64 cursor-not-allowed rounded-xl bg-white py-2.5 pl-10 pr-3 text-sm text-ink shadow-[0_1px_2px_rgba(16,24,40,0.04)] ring-1 ring-line outline-none transition placeholder:text-slate-400 lg:w-72" />
        </label>

        {/* Stub: there is no in-app inbox yet. Events are delivered by e-mail, webhook or Slack (Settings > Notifications). */}
        <DropdownRoot>
          <DropdownTrigger className="!gap-0 !rounded-full !p-2.5 [&>svg:last-child]:hidden">
            <span className="relative"><Bell className="size-5 text-slate-600" />
              {unread > 0 && <><span className="absolute -right-0.5 -top-0.5 size-2.5 animate-[ping-soft_1.8s_ease-out_infinite] rounded-full bg-bad/70" /><span className="absolute -right-0.5 -top-0.5 size-2.5 rounded-full bg-bad ring-2 ring-white" /></>}
            </span>
            <span className="sr-only">{unread ? `${unread} unread notifications` : "Notifications"}</span>
          </DropdownTrigger>
          <DropdownPanel align="end">
            <div className="w-72 p-3">
              <p className="text-sm font-semibold">Notifications</p>
              <p className="mt-1 text-xs text-muted">The in-app inbox is coming soon. Until then, events such as finished reports and cloud connections that need attention are delivered by e-mail, webhook or Slack.</p>
              <Link href="/settings" className="mt-3 inline-block text-xs font-semibold text-primary hover:underline">Set up delivery →</Link>
            </div>
          </DropdownPanel>
        </DropdownRoot>

        <DropdownRoot>
          <DropdownTrigger className="!border-0 !p-1.5 !shadow-none !ring-0 hover:!bg-slate-50">
            <span className="flex size-10 items-center justify-center rounded-full bg-gradient-to-br from-[#2b3a67] to-[#4a63c9] text-sm font-bold text-white shadow-md shadow-indigo-900/20">{initials || "U"}</span>
            <span className="hidden text-left leading-tight sm:block"><span className="block text-sm font-semibold">{user.name}</span><span className="block text-xs capitalize text-muted">{user.role}</span></span>
          </DropdownTrigger>
          <DropdownPanel align="end">
            <DropdownItem asChild><Link href="/settings"><SettingsIcon className="size-4" />{t("header.settings")}</Link></DropdownItem>
            <DropdownItem asChild><a href="/auth/logout"><LogOut className="size-4" />{t("header.sign_out")}</a></DropdownItem>
          </DropdownPanel>
        </DropdownRoot>
      </div>
    </header>
  );
}
