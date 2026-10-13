import type { DxIconName } from "@/components/dx/icons";
import { canBuy, canManageTenant, canSeeNotifications, canSeeSettings, type Role } from "@/lib/permissions";

export type SettingsSection = { id: string; label: string; hint: string; icon: DxIconName; show: boolean };
export type SettingsGroup = { id: string; label: string; items: SettingsSection[] };

/** Addresses of the earlier layout keep working: they land on the section that now holds the content. */
export const SETTINGS_ALIAS: Record<string, string> = { profile: "preferences", members: "users", roles: "users", keys: "security", history: "security", projects: "policies", general: "data", overview: "organization" };

type T = (key: string, vars?: Record<string, string | number>, fallback?: string) => string;

/** One list for the side menu and the page: the sections the role may see, in logical groups. Role checks only decide what to show; the API enforces them again. */
export function settingsGroups(role: Role, t: T): SettingsGroup[] {
  const s = (id: string, labelKey: string, icon: DxIconName, show: boolean): SettingsSection => ({ id, label: t(labelKey), hint: t(`settings.tabdesc.${id}`), icon, show });
  const groups: SettingsGroup[] = [
    { id: "workspace", label: t("dx.settings.group.workspace", undefined, "Workspace"), items: [
      s("organization", "settings.tab.organization", "layers", true),
      s("users", "settings.tab.users", "user", canManageTenant(role)),
    ] },
    { id: "rules", label: t("dx.settings.group.rules", undefined, "Data and rules"), items: [
      s("data", "settings.tab.data", "database", canSeeSettings(role)),
      s("policies", "settings.tab.policies", "sliders", true),
      s("automation", "settings.tab.automation", "automation", true),
    ] },
    { id: "alerts", label: t("dx.settings.group.alerts", undefined, "Alerts"), items: [
      s("notifications", "settings.tab.notifications", "alerts", canSeeNotifications(role)),
    ] },
    { id: "access", label: t("dx.settings.group.access", undefined, "Access"), items: [
      s("security", "settings.tab.security", "shield", canManageTenant(role)),
    ] },
    { id: "account", label: t("dx.settings.group.account", undefined, "Account"), items: [
      s("plan", "settings.tab.plan_billing", "cost", canSeeSettings(role) || canBuy(role)),
      s("preferences", "settings.tab.preferences", "sun", true),
    ] },
  ];
  return groups.map((g) => ({ ...g, items: g.items.filter((i) => i.show) })).filter((g) => g.items.length > 0);
}
