import { redirect } from "next/navigation";
import type { ReactNode } from "react";
import { AppShell } from "@/components/layout/app-shell";
import { checkMembership, getOperator } from "@/lib/api/server";
import { getUser } from "@/lib/auth/session";

export const dynamic = "force-dynamic"; // always evaluated per request: depends on the session

export default async function DashboardLayout({ children }: { children: ReactNode }) {
  const user = await getUser();
  if (!user) redirect("/login");
  if ((await checkMembership()) === "onboarding") redirect("/onboarding"); // signed in but not part of any organization yet
  const isOperator = (await getOperator()) !== null; // the API decides, from the database
  return <AppShell user={user} isOperator={isOperator}>{children}</AppShell>;
}
