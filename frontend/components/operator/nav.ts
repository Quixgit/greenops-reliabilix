import type { DxIconName } from "@/components/dx/icons";

export type NavItem = { href: string; label: string; icon: DxIconName; hint: string; planned?: boolean };
export type NavGroup = { title: string; items: NavItem[] };

/** The console menu. Items marked `planned` open a page that says what is there today and what is next. */
export const OPERATOR_NAV: NavGroup[] = [
  { title: "Overview", items: [
    { href: "/operator", label: "Overview", icon: "overview", hint: "What needs attention now" },
    { href: "/operator/metrics", label: "Metrics", icon: "forecast", hint: "What was created, and when" },
    { href: "/operator/announcements", label: "Announcements", icon: "alerts", hint: "Updates and notices to organizations" },
  ] },
  { title: "Customers", items: [
    { href: "/operator/organizations", label: "Organizations", icon: "user", hint: "Tenants, plans, limits" },
    { href: "/operator/plans", label: "Plans & billing", icon: "cost", hint: "Plan catalog, limits, modules" },
  ] },
  { title: "Data", items: [
    { href: "/operator/data-sources", label: "Data sources", icon: "database", hint: "Sources, services, keys and settings" },
    { href: "/operator/aggregation", label: "Aggregation & policies", icon: "layers", hint: "Source priority, freshness, provenance" },
    { href: "/operator/methodology", label: "Methodology & SCI", icon: "scale", hint: "Versions, coefficients, provenance" },
  ] },
  { title: "Operations", items: [
    { href: "/operator/errors", label: "Errors", icon: "anomaly", hint: "Grouped errors of the services" },
    { href: "/operator/notifications", label: "Notifications", icon: "alerts", hint: "Deliveries, retries" },
    { href: "/operator/connections", label: "Connections", icon: "cloud", hint: "Customer cloud accounts needing attention" },
    { href: "/operator/system", label: "System", icon: "storage", hint: "Build, schema, integration state" },
    { href: "/operator/jobs", label: "Jobs & scheduler", icon: "clock", hint: "Queues, schedules, failed tasks" },
    { href: "/operator/diagnostics", label: "Diagnostics", icon: "intensity", hint: "Self-tests of the platform's services" },
  ] },
  { title: "Governance", items: [
    { href: "/operator/settings", label: "Platform settings", icon: "sliders", hint: "Limits, modules, security, retention" },
    { href: "/operator/audit", label: "Audit log", icon: "reports", hint: "What operators looked at and changed" },
    { href: "/operator/access", label: "Access & security", icon: "shield", hint: "Operators, roles" },
  ] },
];

export const ALL_ITEMS: NavItem[] = OPERATOR_NAV.flatMap((g) => g.items);

/** The item whose page the path belongs to (longest matching prefix; "/operator" only matches itself). */
export function activeItem(path: string): NavItem | undefined {
  return ALL_ITEMS.filter((i) => (i.href === "/operator" ? path === i.href : path === i.href || path.startsWith(i.href + "/")))
    .sort((a, b) => b.href.length - a.href.length)[0];
}

/** The message key of a menu item: "/operator/data-sources" -> "operator.nav.data_sources", "/operator" -> "operator.nav.overview". */
export function navKey(href: string): string {
  const last = href.split("/").filter(Boolean).slice(1).pop() ?? "overview";
  return `operator.nav.${last.replace(/\W+/g, "_")}`;
}
