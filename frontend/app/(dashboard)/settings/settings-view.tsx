"use client";

import { useEffect, useMemo, useState, type ReactNode } from "react";
import { DataTab } from "@/components/settings/data-tab";
import { AuditPanel } from "@/components/settings/audit-panel";
import { AutomationTab } from "@/components/settings/automation-tab";
import { BudgetsPanel } from "@/components/forms/budgets-panel";
import { useFinopsSummary } from "@/hooks/use-overview";
import { ApiKeysPanel } from "@/components/settings/api-keys-panel";
import { SsoPanel } from "@/components/settings/sso-panel";
import { SupportAccessPanel } from "@/components/settings/support-access-panel";

import { MembersPanel } from "@/components/settings/members-panel";
import { NotificationsPanel } from "@/components/settings/notifications-panel";
import { OrganizationTab } from "@/components/settings/organization-tab";
import { BillingTab } from "@/components/billing/billing-tab";
import { PreferencesPanel } from "@/components/settings/preferences-panel";
import { ProjectsPolicyPanel } from "@/components/settings/projects-policy-panel";
import { RolesPanel } from "@/components/settings/roles-panel";
import { SettingsHistory } from "@/components/settings/settings-editor";
import { LatencyPanel } from "@/components/settings/latency-panel";
import { Card } from "@/components/ui/card";
import { Pill } from "@/components/operator/ui";
import { SETTINGS_ALIAS, settingsGroups } from "@/components/settings/sections";
import { useT } from "@/i18n/provider";
import { canBuy, canManageTenant, canSeeSettings, canWrite, type Role } from "@/lib/permissions";
import { cn } from "@/lib/utils";

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

/** Role checks here only decide what to show; the Go API enforces every one of them again. */
export function SettingsView({ user }: { user: { name: string; email: string; role: Role } }) {
  const t = useT();
  const groups = useMemo(() => settingsGroups(user.role, t), [t, user.role]);
  const tabs = useMemo(() => groups.flatMap((g) => g.items), [groups]);

  const [active, setActive] = useState("organization");
  // The tab is in the address (#users) so a link or a reload lands on it.
  useEffect(() => {
    const read = () => {
      const h = window.location.hash.slice(1);
      const id = SETTINGS_ALIAS[h] ?? h;
      if (id && tabs.some((x) => x.id === id)) setActive(id);
    };
    read();
    window.addEventListener("hashchange", read); // a link or the back button to another #tab while this page is open
    return () => window.removeEventListener("hashchange", read);
  }, [tabs]);
  const go = (id: string) => {
    const target = SETTINGS_ALIAS[id] ?? id;
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
        <LatencyPanel canEdit={edit} />
        <BudgetsPanel canEdit={canManageTenant(user.role) || edit} currency={cost.data?.currency ?? "USD"} />
      </div>
    ),
    automation: <AutomationTab go={go} />,
    notifications: <NotificationsPanel canEdit={edit} />,
    security: (
      <div className="space-y-4">
        <SsoPanel canEdit={canManageTenant(user.role)} />
        <ApiKeysPanel />
        <SupportAccessPanel />
        <AuditPanel />
        {canSeeSettings(user.role) && <SettingsHistory path="settings/history?limit=100" />}
      </div>
    ),
    plan: <BillingTab canBuy={canBuy(user.role)} />,
    preferences: <div className="space-y-4"><ProfileCard user={user} /><PreferencesPanel /></div>,
  };

  return (
    <div className="mx-auto max-w-[1280px] space-y-6">
      <div className="rise">
        <p className="text-xs font-semibold uppercase tracking-[0.14em] text-muted">{t("settings.title", undefined, "Settings")}</p>
        <h1 className="mt-1 text-2xl font-bold tracking-tight">{current.label}</h1>
        <p className="mt-0.5 text-sm text-muted">{current.hint}</p>
      </div>

      {/* Below 1024 px the side menu is not there (it slides out beside the main menu on wide screens), so the sections are a strip here. */}
      <nav role="tablist" aria-label="Settings" className="flex gap-1 overflow-x-auto border-b border-line lg:hidden [scrollbar-width:none]">
        {tabs.map((x) => (
          <button key={x.id} role="tab" type="button" aria-selected={active === x.id} onClick={() => go(x.id)}
            className={cn("shrink-0 border-b-2 px-3 py-2 text-sm font-semibold", active === x.id ? "border-[var(--dx-navy)] text-ink" : "border-transparent text-muted")}>{x.label}</button>
        ))}
      </nav>

      <section key={current.id} role="tabpanel" aria-label={current.label} className="page-enter settings-skin min-w-0">
        {body[current.id]}
      </section>
    </div>
  );
}
