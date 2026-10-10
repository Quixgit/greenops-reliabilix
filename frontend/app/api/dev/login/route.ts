import { NextRequest, NextResponse } from "next/server";
import { DEV_COOKIE, credentialsOk, devLoginEnabled, sessionValue } from "@/lib/auth/devsession";
import { createLimiter, safeReturnTo } from "@/lib/auth/login";

// Development sign-in (see lib/auth/devsession.ts). 404 unless that mode is on, so it does not exist in production.
const allow = createLimiter(10, 60_000);

export async function POST(req: NextRequest) {
  if (!devLoginEnabled()) return new NextResponse(null, { status: 404 });
  const ip = req.headers.get("x-forwarded-for")?.split(",")[0]?.trim() || "unknown";
  const base = process.env.APP_BASE_URL || req.nextUrl.origin;
  const form = await req.formData();
  const returnTo = safeReturnTo(form.get("returnTo"));
  if (!allow(ip)) return NextResponse.redirect(new URL("/login?error=rate", base), 303);
  const user = String(form.get("username") ?? "").slice(0, 100);
  const password = String(form.get("password") ?? "").slice(0, 200);
  if (!credentialsOk(user, password)) return NextResponse.redirect(new URL("/login?error=1", base), 303);
  const res = NextResponse.redirect(new URL(returnTo, base), 303);
  res.cookies.set(DEV_COOKIE, await sessionValue(), { httpOnly: true, sameSite: "lax", path: "/", maxAge: 12 * 3600, secure: base.startsWith("https://") });
  return res;
}
