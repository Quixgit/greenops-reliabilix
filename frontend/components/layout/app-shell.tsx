import type { ReactNode } from "react";
import type { AppUser } from "@/lib/auth/session";
import { Header } from "./header";
import { Sidebar } from "./sidebar";

export function AppShell({ user, isOperator = false, children }: { user: AppUser; isOperator?: boolean; children: ReactNode }) {
  return (
    <div className="min-h-screen lg:pl-[236px]">
      <Sidebar isOperator={isOperator} />
      <Header user={user} />
      <main className="min-w-0 overflow-x-clip p-4 lg:p-6">{children}</main>
    </div>
  );
}
