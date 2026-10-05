"use client";

import { Bell, Menu, Search, Settings as SettingsIcon, LogOut } from "lucide-react";
import Link from "next/link";
import { Logo } from "./logo";
import { DropdownItem, DropdownPanel, DropdownRadioGroup, DropdownRadioItem, DropdownRoot, DropdownTrigger } from "@/components/ui/dropdown";
import { useProjects } from "@/hooks/use-overview";
import { PERIODS, useUi, type PeriodKey } from "@/lib/store";
import type { Role } from "@/lib/permissions";

const ALL = "__all__";

export function Header({ user, unread = 0 }: { user: { name: string; email: string; role: Role }; unread?: number }) {
  const { projectId, setProject, period, setPeriod, setNavOpen } = useUi();
  const projects = useProjects();
  const selected = projects.data?.find((p) => p.id === projectId);

  return (
    <header className="sticky top-0 z-50 flex items-center gap-3 border-b border-white/70 bg-white/60 px-4 py-3 backdrop-blur-xl lg:px-6">
      <button className="rounded-lg p-2 hover:bg-white lg:hidden" aria-label="Open menu" onClick={() => setNavOpen(true)}><Menu className="size-5" /></button>
      <Link href="/overview" className="flex items-center gap-2.5">
        <Logo />
        <span className="leading-tight"><span className="block text-base font-bold tracking-tight">Reliabilix</span><span className="block text-[11px] text-muted">GreenOps Platform</span></span>
      </Link>

      <div className="ml-4 hidden items-center gap-2 md:flex">
        <DropdownRoot>
          <DropdownTrigger>{selected?.name ?? "All Projects"}</DropdownTrigger>
          <DropdownPanel>
            <DropdownRadioGroup value={projectId ?? ALL} onValueChange={(v) => setProject(v === ALL ? null : v)}>
              <DropdownRadioItem value={ALL}>All Projects</DropdownRadioItem>
              {projects.data?.map((p) => <DropdownRadioItem key={p.id} value={p.id}>{p.name}</DropdownRadioItem>)}
            </DropdownRadioGroup>
          </DropdownPanel>
        </DropdownRoot>
        <DropdownRoot>
          <DropdownTrigger>{PERIODS.find((p) => p.key === period)!.label}</DropdownTrigger>
          <DropdownPanel>
            <DropdownRadioGroup value={period} onValueChange={(v) => setPeriod(v as PeriodKey)}>
              {PERIODS.map((p) => <DropdownRadioItem key={p.key} value={p.key}>{p.label}</DropdownRadioItem>)}
            </DropdownRadioGroup>
          </DropdownPanel>
        </DropdownRoot>
      </div>

      <div className="ml-auto flex items-center gap-3">
        {/* Search arrives with a later release: the field is present but intentionally read-only. */}
        <label className="relative hidden xl:block">
          <Search className="absolute left-3 top-1/2 size-4 -translate-y-1/2 text-muted" />
          <input readOnly aria-label="Search (coming soon)" title="Search is coming soon" placeholder="Search…"
            className="w-56 cursor-not-allowed rounded-xl bg-white/80 py-2 pl-9 pr-3 text-sm ring-1 ring-line outline-none" />
        </label>
        <button className="relative rounded-xl bg-white/80 p-2.5 ring-1 ring-line hover:bg-white" aria-label={unread ? `${unread} unread notifications` : "Notifications"}>
          <Bell className="size-[18px]" />
          {unread > 0 && <span className="absolute -right-1 -top-1 flex size-4 items-center justify-center rounded-full bg-bad text-[10px] font-bold text-white">{unread}</span>}
        </button>
        <DropdownRoot>
          <DropdownTrigger className="py-1.5">
            <span className="brand-gradient flex size-8 items-center justify-center rounded-full text-xs font-bold text-white">{user.name.slice(0, 1).toUpperCase()}</span>
            <span className="hidden text-left leading-tight sm:block"><span className="block text-sm font-semibold">{user.name}</span><span className="block text-[11px] capitalize text-muted">{user.role}</span></span>
          </DropdownTrigger>
          <DropdownPanel align="end">
            <DropdownItem asChild><Link href="/settings"><SettingsIcon className="size-4" />Settings</Link></DropdownItem>
            <DropdownItem asChild><a href="/auth/logout"><LogOut className="size-4" />Sign out</a></DropdownItem>
          </DropdownPanel>
        </DropdownRoot>
      </div>
    </header>
  );
}
