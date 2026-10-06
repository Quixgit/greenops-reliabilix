import "server-only";
import { auth0 } from "@/lib/auth0";
import { devAuthEnabled } from "./dev";
import type { Role } from "@/lib/permissions";

export type AppUser = { name: string; email: string; picture?: string; role: Role };

const ROLES: Role[] = ["owner", "admin", "engineer", "viewer", "billing"];
const asRole = (v: unknown): Role => (ROLES.includes(v as Role) ? (v as Role) : "viewer");

/** Returns the signed-in user or null. Never exposes tokens to client components. */
export async function getUser(): Promise<AppUser | null> {
  if (devAuthEnabled()) {
    return { name: "Dev User", email: "dev@reliabilix.local", role: asRole(process.env.DEV_ROLE ?? "owner") };
  }
  const session = await auth0.getSession();
  if (!session) return null;
  const ns = process.env.AUTH_CLAIM_NS ?? "https://reliabilix.com/";
  const u = session.user as Record<string, unknown>;
  return {
    name: String(u.name ?? u.email ?? "User"),
    email: String(u.email ?? ""),
    picture: typeof u.picture === "string" ? u.picture : undefined,
    role: asRole(u[`${ns}role`]),
  };
}

/** Bearer token for the Go API, resolved server-side only. */
export async function getApiToken(): Promise<string | null> {
  if (devAuthEnabled()) {
    const tenant = process.env.DEV_TENANT_ID;
    if (!tenant) return null;
    return `dev:dev-user:${tenant}:${process.env.DEV_ROLE ?? "owner"}`;
  }
  try {
    const { token } = await auth0.getAccessToken();
    return token ?? null;
  } catch {
    return null;
  }
}
