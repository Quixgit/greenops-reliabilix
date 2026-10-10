import { NextRequest, NextResponse } from "next/server";
import { DEV_COOKIE } from "@/lib/auth/devsession";

// Reached only in the development sign-in mode: with Auth0 the proxy handles /auth/logout before any route.
export async function GET(req: NextRequest) {
  const base = process.env.APP_BASE_URL || req.nextUrl.origin;
  const res = NextResponse.redirect(new URL("/login", base), 303);
  res.cookies.delete(DEV_COOKIE);
  return res;
}
