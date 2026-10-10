"use client";

import { Bell, Building2, CreditCard, Database, FolderKanban, History, KeyRound, Search, ShieldCheck, SlidersHorizontal, UserRound, Users, type LucideIcon } from "lucide-react";
import { useEffect, useMemo, useState, type ReactNode } from "react";
import { ApiKeysPanel } from "@/components/settings/api-keys-panel";
import { DataPanel } from "@/components/settings/data-panel";
import { MembersPanel } from "@/components/settings/members-panel";
import { NotificationsPanel } from "@/components/settings/notifications-panel";
import { PlanPanel } from "@/components/settings/plan-panel";
import { PreferencesPanel } from "@/components/settings/preferences-panel";
import { ProjectsPolicyPanel } from "@/components/settings/projects-policy-panel";
import { RolesPanel } from "@/components/settings/roles-panel";
import { SettingsEditor, SettingsHistory } from "@/components/settings/settings-editor";
import { Card, CardHeader } from "@/components/ui/card";
import { Pill } from "@/components/operator/ui";
import { useT } from "@/i18n/provider";
import { canManageTenant, canSeeNotifications, canSeeSettings, canWrite, type Role } from "@/lib/permissions";
import { cn } from "@/lib/utils";

type Section = { id: string; group: string; label: string; hint: string; icon: LucideIcon; show: boolean };

const CAPABILITIES: [string, (r: Role) => boolean][] = [
  ["See the dashboards, recommendations and reports", () => true],
  ["Change organization settings and project policies", canWrite],
  ["Manage notification destinations", canWrite],
  ["Invite members and issue API keys", canManageTenant],
  ["Buy or change the plan", (r) => r === "owner" || r === "billing" || r === "admin"],
  ["Export or erase the organization's data", (r) => r === "owner"],
];

function ProfileCard({ user }: { user: { name: string; email: string; role: Role } }) {
  const initials = (user.name.match(/[A-Za-z0-9]/g) ?? ["U"]).slice(0, 2).join("").toUpperCase();
  return (
    <Card interactive={false}>
      <div className="flex items-center gap-4">
        <span className="flex size-14 items-center justify-center rounded-full bg-gradient-to-br from-[#2b3a67] to-[#4a63c9] text-lg font-bold text-white shadow-md shadow-indigo-900/20">{initials}</span>
        <div className="min-w-0">
          <p className="truncate text-lg font-semibold">{user.name}</p>
          <p className="truncate text-sm text-muted">{user.email || "—"}</p>
        </div>
        <span className="ml-auto"><Pill tone={user.role === "owner" ? "violet" : user.role === "admin" ? "info" : "neutral"}>{user.role}</Pill></span>
      </div>
      <p className="mb-2 mt-5 text-xs font-semibold uppercase tracking-wide text-muted">What your role can do</p>
      <ul className="grid gap-x-6 gap-y-1.5 text-sm sm:grid-cols-2">
        {CAPABILITIES.map(([text, can]) => (
          <li key={text} className={cn("flex items-start gap-2", !can(user.role) && "text-slate-400")}>
            <span className={cn("mt-0.5 grid size-4 shrink-0 place-items-center rounded-full text-[10px] font-bold", can(user.role) ? "bg-emerald-100 text-emerald-700" : "bg-slate-100 text-slate-400")}>{can(user.role) ? "✓" : "–"}</span>{text}
          </li>
        ))}
      </ul>
    </Card>
  );
}

/** Role checks here only decide what to show; the Go API enforces every one of them again. */
export function SettingsView({ user }: { user: { name: string; email: string; role: Role } }) {
  const t = useT();
  const sections: Section[] = useMemo(() => [
    { id: "profile", group: "Account", label: "Profile", hint: "You and your role", icon: UserRound, show: true },
    { id: "preferences", group: "Account", label: t("settings.prefs.title", undefined, "Preferences"), hint: "Period, units, density, motion", icon: SlidersHorizontal, show: true },
    { id: "organization", group: "Organization", label: "General", hint: "Retention, modules, defaults", icon: Building2, show: canSeeSettings(user.role) },
    { id: "projects", group: "Organization", label: "Projects & policies", hint: "Regions, automation, limits", icon: FolderKanban, show: true },
    { id: "notifications", group: "Organization", label: "Notifications", hint: "E-mail, webhooks, Slack", icon: Bell, show: canSeeNotifications(user.role) },
    { id: "members", group: "Access", label: "Members", hint: "People and invitations", icon: Users, show: canManageTenant(user.role) },
    { id: "roles", group: "Access", label: t("settings.tab.roles"), hint: "What each role may do", icon: ShieldCheck, show: canManageTenant(user.role) },
    { id: "keys", group: "Access", label: "API keys", hint: "CI and automation", icon: KeyRound, show: canManageTenant(user.role) },
    { id: "plan", group: "Billing", label: t("settings.tab.plan"), hint: "Plan, limits, payment", icon: CreditCard, show: canSeeSettings(user.role) },
    { id: "history", group: "Data", label: "Change history", hint: "Who changed what", icon: History, show: canSeeSettings(user.role) },
    { id: "data", group: "Data", label: "Data & privacy", hint: "Export and erasure", icon: Database, show: user.role === "owner" },
  ].filter((s) => s.show), [t, user.role]);

  const [active, setActive] = useState("profile");
  const [query, setQuery] = useState("");
  // The section is in the address (#members) so a link or a reload lands on it.
  useEffect(() => {
    const h = window.location.hash.slice(1);
    if (h && sections.some((s) => s.id === h)) setActive(h);
  }, [sections]);
  const go = (id: string) => { setActive(id); window.history.replaceState(null, "", `#${id}`); };

  const visible = sections.filter((s) => !query || `${s.label} ${s.hint} ${s.group}`.toLowerCase().includes(query.toLowerCase()));
  const groups = Array.from(new Set(visible.map((s) => s.group)));
  const current = sections.find((s) => s.id === active) ?? sections[0];

  const body: Record<string, ReactNode> = {
    profile: <ProfileCard user={user} />,
    preferences: <PreferencesPanel />,
    organization: <SettingsEditor mode="tenant" path="settings" historyPath="settings/history" canEdit={canWrite(user.role)} />,
    projects: <ProjectsPolicyPanel canEdit={canWrite(user.role)} />,
    notifications: <NotificationsPanel canEdit={canWrite(user.role)} />,
    members: <MembersPanel />,
    roles: <RolesPanel />,
    keys: <ApiKeysPanel />,
    plan: <PlanPanel canBuy={canManageTenant(user.role)} />,
    history: <SettingsHistory path="settings/history?limit=100" />,
    data: <DataPanel />,
  };

  return (
    <div className="mx-auto max-w-[1180px] space-y-5">
      <div className="rise">
        <h1 className="text-2xl font-bold tracking-tight">Settings</h1>
        <p className="mt-1 text-sm text-muted">Your account, your organization and how the platform behaves for it.</p>
      </div>
      <div className="grid gap-5 lg:grid-cols-[240px_minmax(0,1fr)]">
        <nav aria-label="Settings sections" className="lg:sticky lg:top-20 lg:self-start">
          <div className="relative mb-3">
            <Search className="pointer-events-none absolute left-3 top-1/2 size-4 -translate-y-1/2 text-muted" aria-hidden />
            <input value={query} onChange={(e) => setQuery(e.target.value)} placeholder="Find a section…" aria-label="Find a section"
              className="w-full rounded-xl bg-white py-2.5 pl-9 pr-3 text-sm ring-1 ring-line outline-none focus:ring-2 focus:ring-primary" />
          </div>
          <div className="thin-scroll flex gap-1 overflow-x-auto lg:block lg:space-y-4 lg:overflow-visible">
            {groups.map((g) => (
              <div key={g} className="flex gap-1 lg:block">
                <p className="mb-1 hidden px-3 text-[11px] font-semibold uppercase tracking-wider text-slate-400 lg:block">{g}</p>
                {visible.filter((s) => s.group === g).map((s) => (
                  <button key={s.id} type="button" onClick={() => go(s.id)} aria-current={active === s.id ? "page" : undefined}
                    className={cn("flex w-full shrink-0 items-center gap-3 whitespace-nowrap rounded-xl px-3 py-2 text-left text-sm font-medium transition lg:whitespace-normal",
                      active === s.id ? "bg-primary/10 text-primary" : "text-slate-600 hover:bg-slate-100")}>
                    <s.icon className="size-[18px] shrink-0" strokeWidth={1.8} />{s.label}
                  </button>
                ))}
              </div>
            ))}
            {visible.length === 0 && <p className="px-3 py-2 text-sm text-muted">No section matches.</p>}
          </div>
        </nav>
        <section key={current.id} className="page-enter min-w-0 space-y-4" aria-labelledby="settings-section-title">
          <header>
            <h2 id="settings-section-title" className="text-lg font-semibold tracking-tight">{current.label}</h2>
            <p className="text-sm text-muted">{current.hint}</p>
          </header>
          {body[current.id]}
        </section>
      </div>
    </div>
  );
}
