"use client";

import { Bell, Building2, CreditCard, Database, KeyRound, Palette, Settings as SettingsIcon, SlidersHorizontal, Users, Workflow, type LucideIcon } from "lucide-react";
import { useEffect, useMemo, useState, type ReactNode } from "react";
import { AutomationTab } from "@/components/settings/automation-tab";
import { ApiKeysPanel } from "@/components/settings/api-keys-panel";
import { DataPanel } from "@/components/settings/data-panel";
import { MembersPanel } from "@/components/settings/members-panel";
import { NotificationsPanel } from "@/components/settings/notifications-panel";
import { OrganizationTab } from "@/components/settings/organization-tab";
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

type Section = { id: string; label: string; icon: LucideIcon; show: boolean };

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

/** Addresses of the earlier layout keep working: they land on the tab that now holds the section. */
const ALIAS: Record<string, string> = { profile: "preferences", members: "users", roles: "users", keys: "security", history: "security", projects: "policies", general: "data", overview: "organization" };

/** Role checks here only decide what to show; the Go API enforces every one of them again. */
export function SettingsView({ user }: { user: { name: string; email: string; role: Role } }) {
  const t = useT();
  const tabs: Section[] = useMemo(() => [
    { id: "organization", label: t("settings.tab.organization"), icon: Building2, show: true },
    { id: "users", label: t("settings.tab.users"), icon: Users, show: canManageTenant(user.role) },
    { id: "data", label: t("settings.tab.data"), icon: Database, show: canSeeSettings(user.role) },
    { id: "policies", label: t("settings.tab.policies"), icon: SlidersHorizontal, show: true },
    { id: "automation", label: t("settings.tab.automation"), icon: Workflow, show: true },
    { id: "notifications", label: t("settings.tab.notifications"), icon: Bell, show: canSeeNotifications(user.role) },
    { id: "security", label: t("settings.tab.security"), icon: KeyRound, show: canManageTenant(user.role) },
    { id: "plan", label: t("settings.tab.plan_billing"), icon: CreditCard, show: canSeeSettings(user.role) },
    { id: "preferences", label: t("settings.tab.preferences"), icon: Palette, show: true },
  ].filter((x) => x.show), [t, user.role]);

  const [active, setActive] = useState("organization");
  // The tab is in the address (#users) so a link or a reload lands on it.
  useEffect(() => {
    const h = window.location.hash.slice(1);
    const id = ALIAS[h] ?? h;
    if (id && tabs.some((x) => x.id === id)) setActive(id);
  }, [tabs]);
  const go = (id: string) => {
    const target = ALIAS[id] ?? id;
    setActive(target);
    window.history.replaceState(null, "", `#${target}`);
  };
  const current = tabs.find((x) => x.id === active) ?? tabs[0];
  const edit = canWrite(user.role);

  const body: Record<string, ReactNode> = {
    organization: <OrganizationTab go={go} canManage={canManageTenant(user.role)} canEdit={edit} />,
    users: <div className="space-y-4"><MembersPanel /><RolesPanel /></div>,
    data: (
      <div className="space-y-4">
        <SettingsEditor mode="tenant" path="settings" historyPath="settings/history" canEdit={edit} />
        {user.role === "owner" && <DataPanel />}
      </div>
    ),
    policies: <ProjectsPolicyPanel canEdit={edit} />,
    automation: <AutomationTab go={go} />,
    notifications: <NotificationsPanel canEdit={edit} />,
    security: (
      <div className="space-y-4">
        <ApiKeysPanel />
        {canSeeSettings(user.role) && <SettingsHistory path="settings/history?limit=100" />}
      </div>
    ),
    plan: <PlanPanel canBuy={canManageTenant(user.role)} />,
    preferences: <div className="space-y-4"><ProfileCard user={user} /><PreferencesPanel /></div>,
  };

  return (
    <div className="mx-auto max-w-[1280px] space-y-5">
      <div className="rise flex items-center gap-4">
        <SettingsIcon className="size-10 text-[#3d5a8a] transition-transform duration-700 hover:rotate-90" strokeWidth={1.4} />
        <div>
          <h1 className="text-2xl font-bold tracking-tight">Settings</h1>
          <p className="mt-0.5 text-sm text-muted">{t("settings.subtitle")}</p>
        </div>
      </div>

      <div role="tablist" aria-label="Settings" className="rise -mb-px flex gap-0 overflow-x-auto border-b border-line [scrollbar-width:none] [&::-webkit-scrollbar]:hidden" style={{ "--i": 1 } as React.CSSProperties}>
        {tabs.map((x) => (
          <button key={x.id} role="tab" type="button" aria-selected={active === x.id} onClick={() => go(x.id)}
            className={cn("group relative flex shrink-0 items-center whitespace-nowrap px-4 py-3 text-sm font-medium transition-colors duration-200",
              active === x.id ? "text-emerald-600" : "text-slate-600 hover:text-ink")}>
            {x.label}
            <span aria-hidden className={cn("absolute inset-x-3 -bottom-px h-0.5 origin-left rounded-full bg-emerald-600 transition-transform duration-300",
              active === x.id ? "scale-x-100" : "scale-x-0 group-hover:scale-x-50 group-hover:bg-slate-300")} />
          </button>
        ))}
      </div>

      <section key={current.id} role="tabpanel" aria-label={current.label} className="page-enter settings-skin min-w-0 pt-5">
        {body[current.id]}
      </section>
    </div>
  );
}
