import { redirect } from "next/navigation";
import type { ReactNode } from "react";
import { AppShell } from "@/components/layout/app-shell";
import { getUser } from "@/lib/auth/session";

export const dynamic = "force-dynamic"; // always evaluated per request: depends on the session

export default async function DashboardLayout({ children }: { children: ReactNode }) {
  const user = await getUser();
  if (!user) redirect("/login");
  return <AppShell user={user}>{children}</AppShell>;
}
