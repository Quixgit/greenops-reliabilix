"use client";

import { AlertTriangle, ArrowLeft, Bell, Cloud, LayoutDashboard, Server, ShieldCheck, SlidersHorizontal, type LucideIcon } from "lucide-react";
import Link from "next/link";
import { usePathname } from "next/navigation";
import type { ReactNode } from "react";
import { Badge } from "@/components/ui/badge";
import { cn } from "@/lib/utils";

const NAV: { href: string; label: string; icon: LucideIcon }[] = [
  { href: "/operator", label: "Organizations", icon: LayoutDashboard },
  { href: "/operator/settings", label: "Platform settings", icon: SlidersHorizontal },
  { href: "/operator/errors", label: "Errors", icon: AlertTriangle },
  { href: "/operator/notifications", label: "Notifications", icon: Bell },
  { href: "/operator/connections", label: "Connections", icon: Cloud },
  { href: "/operator/system", label: "System", icon: Server },
];

export function OperatorShell({ operator, children }: { operator: { email: string; subject: string; role: string }; children: ReactNode }) {
  const path = usePathname();
  return (
    <div className="min-h-screen">
      <header className="sticky top-0 z-50 flex flex-wrap items-center gap-3 border-b border-white/70 bg-slate-900 px-4 py-3 text-white lg:px-6">
        <ShieldCheck className="size-5 text-emerald-300" aria-hidden />
        <span className="font-bold tracking-tight">Operator console</span>
        <Badge tone={operator.role === "admin" ? "good" : "neutral"}>{operator.role}</Badge>
        <span className="ml-auto flex items-center gap-4 text-sm text-slate-300">
          <span className="hidden sm:inline">{operator.email || operator.subject}</span>
          <Link href="/overview" className="inline-flex items-center gap-1 rounded-lg px-2.5 py-1.5 text-white ring-1 ring-white/25 hover:bg-white/10"><ArrowLeft className="size-4" />Back to app</Link>
        </span>
      </header>
      <div className="mx-auto flex max-w-[1600px] flex-col gap-4 p-4 lg:flex-row lg:p-6">
        <nav aria-label="Operator" className="flex shrink-0 gap-1 overflow-x-auto lg:w-56 lg:flex-col">
          {NAV.map(({ href, label, icon: Icon }) => {
            const active = href === "/operator" ? path === href || path.startsWith("/operator/organizations") : path.startsWith(href);
            return (
              <Link key={href} href={href} aria-current={active ? "page" : undefined}
                className={cn("flex items-center gap-3 whitespace-nowrap rounded-xl px-3.5 py-2.5 text-sm font-medium transition", active ? "brand-gradient text-white shadow-md shadow-indigo-500/25" : "text-slate-600 hover:bg-white/70")}>
                <Icon className="size-[18px]" />{label}
              </Link>
            );
          })}
        </nav>
        <main className="min-w-0 flex-1">{children}</main>
      </div>
    </div>
  );
}
