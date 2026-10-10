import {
  AlertTriangle, Bell, Boxes, Calculator, Cloud, Database, GitMerge, History, LayoutDashboard, Plug, Receipt, Server, SlidersHorizontal,
  Timer, Users, type LucideIcon,
} from "lucide-react";

export type NavItem = { href: string; label: string; icon: LucideIcon; hint: string; planned?: boolean };
export type NavGroup = { title: string; items: NavItem[] };

/** The console menu. Items marked `planned` open a page that says what is there today and what is next. */
export const OPERATOR_NAV: NavGroup[] = [
  { title: "Overview", items: [
    { href: "/operator", label: "Overview", icon: LayoutDashboard, hint: "What needs attention now" },
  ] },
  { title: "Customers", items: [
    { href: "/operator/organizations", label: "Organizations", icon: Users, hint: "Tenants, plans, limits" },
    { href: "/operator/plans", label: "Plans & billing", icon: Receipt, hint: "Plan catalog, limits, modules" },
  ] },
  { title: "Data", items: [
    { href: "/operator/data-sources", label: "Data sources", icon: Database, hint: "Sources, services, keys and settings" },
        { href: "/operator/planned/aggregation", label: "Aggregation & policies", icon: GitMerge, hint: "Source priority, fallback, provenance", planned: true },
    { href: "/operator/methodology", label: "Methodology & SCI", icon: Calculator, hint: "Versions, coefficients, provenance" },
  ] },
  { title: "Operations", items: [
    { href: "/operator/errors", label: "Errors", icon: AlertTriangle, hint: "Grouped errors of the services" },
    { href: "/operator/notifications", label: "Notifications", icon: Bell, hint: "Deliveries, retries" },
    { href: "/operator/connections", label: "Connections", icon: Cloud, hint: "Customer cloud accounts needing attention" },
    { href: "/operator/system", label: "System", icon: Server, hint: "Build, schema, integration state" },
    { href: "/operator/jobs", label: "Jobs & scheduler", icon: Timer, hint: "Queues, schedules, failed tasks" },
  ] },
  { title: "Governance", items: [
    { href: "/operator/settings", label: "Platform settings", icon: SlidersHorizontal, hint: "Limits, modules, security, retention" },
    { href: "/operator/audit", label: "Audit log", icon: History, hint: "What operators looked at and changed" },
    { href: "/operator/access", label: "Access & security", icon: Boxes, hint: "Operators, roles" },
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
