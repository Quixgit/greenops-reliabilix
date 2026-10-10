import type { NextRequest } from "next/server";
import { NextResponse } from "next/server";
import { auth0 } from "./lib/auth0";
import { basicAuthOk } from "./lib/auth/basic";
import { devAuthEnabled } from "./lib/auth/dev";

// Next.js 16 "proxy" (formerly middleware): mounts /auth/login, /auth/logout, /auth/callback.
export async function proxy(request: NextRequest) {
  if (devAuthEnabled()) {
    // Dev auth signs everybody in. If such an instance is reachable from outside, DEV_BASIC_AUTH puts a password in front.
    const gate = process.env.DEV_BASIC_AUTH;
    if (gate && !basicAuthOk(request.headers.get("authorization"), gate)) {
      return new NextResponse("Authentication required", { status: 401, headers: { "WWW-Authenticate": 'Basic realm="Reliabilix dev", charset="UTF-8"' } });
    }
    return NextResponse.next();
  }
  return auth0.middleware(request);
}

export const config = {
  matcher: ["/((?!_next/static|_next/image|favicon.ico|mountains.svg).*)"],
};
