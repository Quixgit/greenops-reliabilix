import { Bell, Boxes, FileText, FolderKanban, GitBranch, LayoutDashboard, Leaf, Lightbulb, Plug, Settings, Workflow, type LucideIcon } from "lucide-react";

// The 12 items of the client workspace (docs/product/01-client-workspace.md). Every route exists, none 404s;
// a page without a backend yet says so instead of showing invented numbers.
export const NAV: { href: string; label: string; icon: LucideIcon }[] = [
  { href: "/overview", label: "Overview", icon: LayoutDashboard },
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
