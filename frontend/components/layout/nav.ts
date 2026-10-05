import { Boxes, Cloud, FileText, FolderKanban, LayoutDashboard, Lightbulb, Settings, Workflow, type LucideIcon } from "lucide-react";

// One entry per domain of the architecture spec. Every route exists (empty state in phase 1), none 404s.
export const NAV: { href: string; label: string; icon: LucideIcon }[] = [
  { href: "/overview", label: "Overview", icon: LayoutDashboard },
  { href: "/projects", label: "Projects", icon: FolderKanban },
  { href: "/cloud", label: "Cloud Accounts", icon: Cloud },
  { href: "/kubernetes", label: "Kubernetes", icon: Boxes },
  { href: "/reports", label: "Reports", icon: FileText },
  { href: "/recommendations", label: "Recommendations", icon: Lightbulb },
  { href: "/automation", label: "Automation", icon: Workflow },
  { href: "/settings", label: "Settings", icon: Settings },
];
