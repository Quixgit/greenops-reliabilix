"use client";

import { useState } from "react";
import { DataPanel } from "@/components/settings/data-panel";
import { ApiKeysPanel } from "@/components/settings/api-keys-panel";
import { MembersPanel } from "@/components/settings/members-panel";
import { PlanPanel } from "@/components/settings/plan-panel";
import { RolesPanel } from "@/components/settings/roles-panel";
import { useT } from "@/i18n/provider";
import { NotificationsPanel } from "@/components/settings/notifications-panel";
import { Card, CardHeader } from "@/components/ui/card";
import { PillTabs } from "@/components/ui/tabs";
import { SettingsEditor, SettingsHistory } from "@/components/settings/settings-editor";
import { canManageTenant, canSeeNotifications, canSeeSettings, canWrite, type Role } from "@/lib/permissions";

/** Role checks here only decide what to show; the Go API enforces every one of them again. */
export function SettingsView({ user }: { user: { name: string; email: string; role: Role } }) {
  const t = useT();
  const tabs = [
    { value: "profile", label: "Profile" },
    ...(canSeeSettings(user.role) ? [{ value: "organization", label: "Organization" }, { value: "plan", label: t("settings.tab.plan") }] : []),
    ...(canSeeNotifications(user.role) ? [{ value: "notifications", label: "Notifications" }] : []),
    ...(canManageTenant(user.role) ? [{ value: "members", label: "Members" }, { value: "roles", label: t("settings.tab.roles") }, { value: "keys", label: "API keys" }] : []),
    ...(canSeeSettings(user.role) ? [{ value: "history", label: "Change history" }] : []),
    ...(user.role === "owner" ? [{ value: "data", label: "Data" }] : []),
  ];
  const [tab, setTab] = useState("profile");
  return (
    <div className="max-w-4xl space-y-5">
      <h1 className="text-2xl font-bold tracking-tight">Settings</h1>
      <PillTabs value={tab} onChange={setTab} tabs={tabs} label="Settings sections" />
      {tab === "profile" && (
        <Card className="max-w-xl">
          <CardHeader title="Profile" />
          <dl className="grid grid-cols-[120px_1fr] gap-y-2 text-sm">
            <dt className="text-muted">Name</dt><dd>{user.name}</dd>
            <dt className="text-muted">Email</dt><dd>{user.email || "—"}</dd>
            <dt className="text-muted">Role</dt><dd className="capitalize">{user.role}</dd>
          </dl>
        </Card>
      )}
      {tab === "organization" && <SettingsEditor mode="tenant" path="settings" historyPath="settings/history" canEdit={canWrite(user.role)} />}
      {tab === "plan" && <PlanPanel canBuy={canManageTenant(user.role)} />}
      {tab === "history" && <SettingsHistory path="settings/history?limit=100" />}
      {tab === "notifications" && <NotificationsPanel canEdit={canWrite(user.role)} />}
      {tab === "members" && <MembersPanel />}
      {tab === "roles" && <RolesPanel />}
      {tab === "keys" && <ApiKeysPanel />}
      {tab === "data" && <DataPanel />}
    </div>
  );
}
