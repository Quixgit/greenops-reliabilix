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
