import "server-only";
import { getApiToken } from "@/lib/auth/session";

export type Membership = "member" | "onboarding" | "unavailable";

/** Server-side check used by layouts: does the signed-in identity belong to a tenant? */
export async function checkMembership(): Promise<Membership> {
  const token = await getApiToken();
  if (!token) return "unavailable";
  try {
    const res = await fetch(`${process.env.API_URL ?? "http://localhost:8080"}/api/v1/me`, {
      headers: { Authorization: `Bearer ${token}` }, cache: "no-store",
    });
    if (res.ok) return "member";
    if (res.status === 403) {
      const body = (await res.json().catch(() => ({}))) as { detail?: string };
      if (body.detail === "onboarding_required") return "onboarding";
    }
  } catch { /* API unreachable: fall through */ }
  return "unavailable";
}

/** Server-side: the platform operator behind this session, or null for everyone else (the API decides, from the database). */
export async function getOperator(): Promise<{ subject: string; email: string; role: "viewer" | "admin" } | null> {
  const token = await getApiToken();
  if (!token) return null;
  try {
    const res = await fetch(`${process.env.API_URL ?? "http://localhost:8080"}/api/v1/operator/me`, {
      headers: { Authorization: `Bearer ${token}` }, cache: "no-store",
    });
    return res.ok ? ((await res.json()) as { subject: string; email: string; role: "viewer" | "admin" }) : null;
  } catch {
    return null; // API unreachable: do not show the console
  }
}

/**
 * Server-side: the currency this organization's money is shown in (what the cost summary resolves "auto" to). Without it a
 * page that does not itself load the cost summary (Connectors, an account's page) would show dollar signs on a bill in
 * another currency. Falls back to USD when nothing can be told.
 */
export async function getOrgCurrency(): Promise<string> {
  const token = await getApiToken();
  if (!token) return "USD";
  const day = (d: Date) => d.toISOString().slice(0, 10);
  const now = Date.now();
  try {
    const res = await fetch(`${process.env.API_URL ?? "http://localhost:8080"}/api/v1/finops/summary?from=${day(new Date(now - 30 * 86_400_000))}&to=${day(new Date(now + 86_400_000))}`, {
      headers: { Authorization: `Bearer ${token}` }, cache: "no-store",
    });
    if (!res.ok) return "USD";
    const c = ((await res.json()) as { currency?: string }).currency;
    return typeof c === "string" && /^[A-Z]{3}$/.test(c) ? c : "USD";
  } catch {
    return "USD";
  }
}
