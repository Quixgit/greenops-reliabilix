import type { NextRequest } from "next/server";
import { NextResponse } from "next/server";
import { auth0 } from "./lib/auth0";
import { devAuthEnabled } from "./lib/auth/dev";

// Next.js 16 "proxy" (formerly middleware): mounts /auth/login, /auth/logout, /auth/callback.
export async function proxy(request: NextRequest) {
  if (devAuthEnabled()) return NextResponse.next();
  return auth0.middleware(request);
}

export const config = {
  matcher: ["/((?!_next/static|_next/image|favicon.ico|mountains.svg).*)"],
};
