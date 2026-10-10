"use client";

import { useState } from "react";
import { ApiKeysPanel } from "@/components/settings/api-keys-panel";
import { MembersPanel } from "@/components/settings/members-panel";
import { NotificationsPanel } from "@/components/settings/notifications-panel";
import { Card, CardHeader } from "@/components/ui/card";
import { PillTabs } from "@/components/ui/tabs";
import { SettingsEditor, SettingsHistory } from "@/components/settings/settings-editor";
import { canManageTenant, canSeeNotifications, canSeeSettings, canWrite, type Role } from "@/lib/permissions";

/** Role checks here only decide what to show; the Go API enforces every one of them again. */
export function SettingsView({ user }: { user: { name: string; email: string; role: Role } }) {
  const tabs = [
    { value: "profile", label: "Profile" },
    ...(canSeeSettings(user.role) ? [{ value: "organization", label: "Organization" }] : []),
    ...(canSeeNotifications(user.role) ? [{ value: "notifications", label: "Notifications" }] : []),
    ...(canManageTenant(user.role) ? [{ value: "members", label: "Members" }, { value: "keys", label: "API keys" }] : []),
    ...(canSeeSettings(user.role) ? [{ value: "history", label: "Change history" }] : []),
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
      {tab === "history" && <SettingsHistory path="settings/history?limit=100" />}
      {tab === "notifications" && <NotificationsPanel canEdit={canWrite(user.role)} />}
      {tab === "members" && <MembersPanel />}
      {tab === "keys" && <ApiKeysPanel />}
    </div>
  );
}
