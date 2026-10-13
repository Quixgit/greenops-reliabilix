import type { AnyIcon as LucideIcon } from "@/components/ui/icons";
import { IconBell as Bell, IconBranch as GitBranch, IconBulb as Lightbulb, IconCluster as Boxes, IconFlow as Workflow, IconFolder as FolderKanban, IconLeaf as Leaf, IconOverview as LayoutDashboard, IconPlug as Plug, IconReport as FileText, IconSliders as Settings } from "@/components/ui/icons";

// The 12 items of the client workspace (docs/product/01-client-workspace.md). Every route exists, none 404s;
// a page without a backend yet says so instead of showing invented numbers.
export const NAV: { href: string; label: string; icon: LucideIcon }[] = [
  { href: "/overview", label: "Overview", icon: LayoutDashboard },
  { href: "/dev-overview", label: "dev-Overview", icon: LayoutDashboard },
  { href: "/projects", label: "Projects", icon: FolderKanban },
  { href: "/kubernetes", label: "Kubernetes", icon: Boxes },
  { href: "/carbon", label: "Carbon & Energy", icon: Leaf },
  { href: "/reports", label: "Reports", icon: FileText },
  { href: "/recommendations", label: "Recommendations", icon: Lightbulb },
  { href: "/automation", label: "Automation", icon: Workflow },
  { href: "/ci-cd", label: "CI/CD", icon: GitBranch },
  { href: "/connectors", label: "Connectors", icon: Plug },
  { href: "/alerts", label: "Alerts", icon: Bell },
  { href: "/settings", label: "Settings", icon: Settings },
];

/** The main menu: the two secondary places (alerts, settings) live in the gear at the foot of the menu. */
export const MAIN_NAV = NAV.filter((i) => i.href !== "/alerts" && i.href !== "/settings");
