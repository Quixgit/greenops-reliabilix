import type { ReactNode } from "react";
import type { AppUser } from "@/lib/auth/session";
import { Header } from "./header";
import { Sidebar } from "./sidebar";

export function AppShell({ user, children }: { user: AppUser; children: ReactNode }) {
  return (
    <div className="min-h-screen">
      <Header user={user} />
      <div className="mx-auto flex max-w-[1600px]">
        <Sidebar />
        <main className="min-w-0 flex-1 p-4 lg:p-6">{children}</main>
      </div>
    </div>
  );
}
