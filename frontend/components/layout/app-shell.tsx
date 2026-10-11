"use client";

import type { CSSProperties, ReactNode } from "react";
import type { AppUser } from "@/lib/auth/session";
import { useUi } from "@/lib/store";
import { Header } from "./header";
import { Sidebar } from "./sidebar";

export function AppShell({ user, isOperator = false, children }: { user: AppUser; isOperator?: boolean; children: ReactNode }) {
  const collapsed = useUi((s) => s.navCollapsed);
  return (
    <div className="sb-shell min-h-screen" style={{ "--sb": collapsed ? "76px" : "236px" } as CSSProperties}>
      <Sidebar isOperator={isOperator} />
      <Header user={user} />
      <main className="min-w-0 overflow-x-clip p-4 lg:p-6">{children}</main>
    </div>
  );
}
