import { notFound, redirect } from "next/navigation";
import type { ReactNode } from "react";
import { OperatorShell } from "@/components/operator/shell";
import { getOperator } from "@/lib/api/server";
import { getUser } from "@/lib/auth/session";

export const dynamic = "force-dynamic";
export const metadata = { title: "Operator console · Reliabilix GreenOps", robots: { index: false } };

// The API decides who is an operator (from the database). Anyone else gets a 404, as if the console did not exist.
export default async function OperatorLayout({ children }: { children: ReactNode }) {
  const user = await getUser();
  if (!user) redirect("/login");
  const op = await getOperator();
  if (!op) notFound();
  return <OperatorShell operator={op} user={user}>{children}</OperatorShell>;
}
