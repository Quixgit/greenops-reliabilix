/** Helpers of the sign-in page. Pure, so they are unit-tested; the page and the route handler import them. */

const EMAIL = /^[^\s@]{1,64}@[^\s@]{1,255}\.[^\s@]{2,}$/;

export function validEmail(v: unknown): v is string {
  return typeof v === "string" && v.length <= 320 && EMAIL.test(v.trim());
}

/** Only app-relative paths are accepted as a return target: never another origin, never protocol-relative. */
export function safeReturnTo(v: unknown, fallback = "/overview"): string {
  if (typeof v !== "string" || !v.startsWith("/") || v.startsWith("//") || v.startsWith("/\\") || v.includes("\n") || v.includes("\r")) return fallback;
  if (v.startsWith("/auth/") || v.startsWith("/login") || v.startsWith("/api/")) return fallback;
  return v.slice(0, 200);
}

/** Query string of the hosted sign-in (/auth/login is the Auth0 SDK's route). `connection` pre-selects a social provider. */
const CONNECTION = /^[A-Za-z0-9][A-Za-z0-9_.-]{0,99}$/;

export function loginUrl(opts: { connection?: string; email?: string; returnTo?: string; signup?: boolean }): string {
  const p = new URLSearchParams();
  if (opts.signup) p.set("screen_hint", "signup");
  if (opts.connection && CONNECTION.test(opts.connection)) p.set("connection", opts.connection);
  if (opts.email && validEmail(opts.email)) p.set("login_hint", opts.email.trim());
  p.set("returnTo", safeReturnTo(opts.returnTo));
  return `/auth/login?${p}`;
}

/** A tiny fixed-window limiter for the recovery form (per process; the identity provider has its own limits too). */
export function createLimiter(max: number, windowMs: number, now: () => number = Date.now) {
  const hits = new Map<string, { n: number; reset: number }>();
  return (key: string): boolean => {
    const t = now();
    const h = hits.get(key);
    if (!h || h.reset <= t) {
      hits.set(key, { n: 1, reset: t + windowMs });
      if (hits.size > 5000) for (const [k, v] of hits) if (v.reset <= t) hits.delete(k);
      return true;
    }
    h.n += 1;
    return h.n <= max;
  };
}
