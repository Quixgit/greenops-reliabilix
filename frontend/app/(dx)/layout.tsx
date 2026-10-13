import { redirect } from "next/navigation";
import type { ReactNode } from "react";
import "@/components/dx/dx.css";
import { DxShell } from "@/components/dx/shell";
import { CurrencyInit } from "@/components/layout/currency-init";
import { checkMembership, getOperator, getOrgCurrency } from "@/lib/api/server";
import { getUser } from "@/lib/auth/session";

export const dynamic = "force-dynamic"; // per request: depends on the session

/** The DX interface: the same session, membership and data as the dashboard, in its own shell and its own look. */
export default async function DxLayout({ children }: { children: ReactNode }) {
  const user = await getUser();
  if (!user) redirect("/login");
  if ((await checkMembership()) === "onboarding") redirect("/onboarding");
  const [operator, currency] = await Promise.all([getOperator(), getOrgCurrency()]);
  return <DxShell user={user} isOperator={operator !== null}><CurrencyInit currency={currency} />{children}</DxShell>;
}
