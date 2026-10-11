"use client";

import { IconCard as CreditCard, IconPalette as Palette, IconBell as Bell, IconBuilding as Building2, IconDatabase as Database, IconFlow as Workflow, IconKey as KeyRound, IconSliders as SettingsIcon, IconSliders as SlidersHorizontal, IconUsers as Users } from "@/components/ui/icons";
import type { AnyIcon as LucideIcon } from "@/components/ui/icons";
import { useEffect, useMemo, useState, type ReactNode } from "react";
import { DataTab } from "@/components/settings/data-tab";
import { AuditPanel } from "@/components/settings/audit-panel";
import { AutomationTab } from "@/components/settings/automation-tab";
import { BudgetsPanel } from "@/components/forms/budgets-panel";
import { useFinopsSummary } from "@/hooks/use-overview";
import { ApiKeysPanel } from "@/components/settings/api-keys-panel";
import { DataPanel } from "@/components/settings/data-panel";
import { MembersPanel } from "@/components/settings/members-panel";
import { NotificationsPanel } from "@/components/settings/notifications-panel";
import { OrganizationTab } from "@/components/settings/organization-tab";
import { BillingTab } from "@/components/billing/billing-tab";
import { PreferencesPanel } from "@/components/settings/preferences-panel";
import { ProjectsPolicyPanel } from "@/components/settings/projects-policy-panel";
import { RolesPanel } from "@/components/settings/roles-panel";
import { SettingsEditor, SettingsHistory } from "@/components/settings/settings-editor";
import { Card, CardHeader } from "@/components/ui/card";
import { Pill } from "@/components/operator/ui";
import { useT } from "@/i18n/provider";
import { canBuy, canManageTenant, canSeeNotifications, canSeeSettings, canWrite, type Role } from "@/lib/permissions";
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
            <span className={cn("mt-0.5 grid size-4 shrink-0 place-items-center rounded-full text-[10px] font-bold", can(user.role) ? "bg-leaf-soft text-leaf-deep" : "bg-slate-100 text-slate-400")}>{can(user.role) ? "✓" : "–"}</span>{text}
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
    { id: "plan", label: t("settings.tab.plan_billing"), icon: CreditCard, show: canSeeSettings(user.role) || canBuy(user.role) },
    { id: "preferences", label: t("settings.tab.preferences"), icon: Palette, show: true },
  ].filter((x) => x.show), [t, user.role]);

  const [active, setActive] = useState("organization");
  // The tab is in the address (#users) so a link or a reload lands on it.
  useEffect(() => {
    const read = () => {
      const h = window.location.hash.slice(1);
      const id = ALIAS[h] ?? h;
      if (id && tabs.some((x) => x.id === id)) setActive(id);
    };
    read();
    window.addEventListener("hashchange", read); // a link or the back button to another #tab while this page is open
    return () => window.removeEventListener("hashchange", read);
  }, [tabs]);
  const go = (id: string) => {
    const target = ALIAS[id] ?? id;
    setActive(target);
    window.history.replaceState(null, "", `#${target}`);
  };
  const current = tabs.find((x) => x.id === active) ?? tabs[0];
  const edit = canWrite(user.role);
  const cost = useFinopsSummary();

  const body: Record<string, ReactNode> = {
    organization: <OrganizationTab go={go} canManage={canManageTenant(user.role)} canEdit={edit} />,
    users: <div className="space-y-4"><MembersPanel /><RolesPanel /></div>,
    data: <DataTab canEdit={edit} isOwner={user.role === "owner"} />,
    policies: (
      <div className="space-y-4">
        <ProjectsPolicyPanel canEdit={edit} />
        <BudgetsPanel canEdit={canManageTenant(user.role) || edit} currency={cost.data?.currency ?? "USD"} />
      </div>
    ),
    automation: <AutomationTab go={go} />,
    notifications: <NotificationsPanel canEdit={edit} />,
    security: (
      <div className="space-y-4">
        <ApiKeysPanel />
        <AuditPanel />
        {canSeeSettings(user.role) && <SettingsHistory path="settings/history?limit=100" />}
      </div>
    ),
    plan: <BillingTab canBuy={canBuy(user.role)} />,
    preferences: <div className="space-y-4"><ProfileCard user={user} /><PreferencesPanel /></div>,
  };

  return (
    <div className="mx-auto max-w-[1280px] space-y-6">
      <div className="rise flex items-center gap-4">
        <SettingsIcon className="size-12 text-[#5b8fdc] transition-transform duration-700 hover:rotate-90" strokeWidth={1.3} />
        <div>
          <h1 className="text-2xl font-bold tracking-tight">{t("settings.title", undefined, "Settings")}</h1>
          <p className="mt-0.5 text-sm text-muted">{t("settings.subtitle")}</p>
        </div>
      </div>

      <div className="grid gap-6 lg:grid-cols-[250px_minmax(0,1fr)]">
        <nav role="tablist" aria-label="Settings" aria-orientation="vertical"
          className="rise flex gap-1 overflow-x-auto pb-1 [scrollbar-width:none] lg:sticky lg:top-24 lg:flex-col lg:self-start lg:overflow-visible lg:pb-0 [&::-webkit-scrollbar]:hidden" style={{ "--i": 1 } as React.CSSProperties}>
          {tabs.map((x) => {
            const on = active === x.id;
            return (
              <button key={x.id} role="tab" type="button" aria-selected={on} onClick={() => go(x.id)}
                className={cn("group relative flex shrink-0 items-center gap-3 rounded-xl px-3 py-2.5 text-left transition-all duration-200 lg:w-full",
                  on ? "bg-leaf-soft/70 text-leaf-deep" : "text-slate-600 hover:bg-slate-100 hover:text-ink")}>
                <span aria-hidden className={cn("absolute inset-y-2 left-0 hidden w-1 rounded-full bg-leaf-deep transition-transform duration-300 lg:block", on ? "scale-y-100" : "scale-y-0")} />
                <x.icon className={cn("size-7 shrink-0 transition-transform duration-300 group-hover:scale-110", on ? "text-leaf-deep" : "text-slate-400")} strokeWidth={1.5} />
                <span className="min-w-0">
                  <span className="block whitespace-nowrap text-sm font-semibold">{x.label}</span>
                  <span className="hidden truncate text-xs font-normal text-muted lg:block">{t(`settings.tabdesc.${x.id}`)}</span>
                </span>
              </button>
            );
          })}
        </nav>

        <section key={current.id} role="tabpanel" aria-label={current.label} className="page-enter settings-skin min-w-0">
          {body[current.id]}
        </section>
      </div>
    </div>
  );
}
