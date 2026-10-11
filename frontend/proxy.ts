import type { NextRequest } from "next/server";
import { NextResponse } from "next/server";
import { auth0 } from "./lib/auth0";
import { basicAuthOk } from "./lib/auth/basic";
import { devAuthEnabled } from "./lib/auth/dev";
import { DEV_COOKIE, devLoginEnabled, sessionOk } from "./lib/auth/devsession";

/**
 * The Content-Security-Policy of a response. With a nonce, a script runs only when it carries it ('strict-dynamic' lets those scripts load
 * the chunks they need); no inline script of an attacker can run. Next reads the nonce from the CSP of the request and puts it on its own
 * inline scripts, so the policy is also set on the forwarded request.
 */
export function cspFor(nonce: string, dev: boolean): string {
  return [
    "default-src 'self'",
    `script-src 'self' 'nonce-${nonce}' 'strict-dynamic'${dev ? " 'unsafe-eval'" : ""}`,
    "style-src 'self' 'unsafe-inline'", // styles are set inline by the components' own positioning; scripts are what matters
    "img-src 'self' data: https:",
    "font-src 'self' data:",
    "connect-src 'self'",
    "object-src 'none'",
    "base-uri 'self'",
    "form-action 'self'",
    "frame-ancestors 'none'",
  ].join("; ");
}

function nonceValue(): string {
  const b = new Uint8Array(16);
  crypto.getRandomValues(b);
  return btoa(String.fromCharCode(...b));
}

// Next.js 16 "proxy" (formerly middleware): mounts /auth/login, /auth/logout, /auth/callback, and sets the CSP with a fresh nonce.
export async function proxy(request: NextRequest) {
  const nonce = nonceValue();
  const csp = cspFor(nonce, process.env.NODE_ENV !== "production");
  const res = await route(request, nonce, csp);
  // The Auth0 response is the SDK's own; it is given the legacy policy in route(), so nothing is overwritten here.
  if (!res.headers.has("Content-Security-Policy")) res.headers.set("Content-Security-Policy", csp);
  return res;
}

/** The legacy policy (inline scripts allowed): used only where the nonce cannot be handed to the page, see route(). */
const LEGACY_CSP = "default-src 'self'; img-src 'self' data: https:; style-src 'self' 'unsafe-inline'; script-src 'self' 'unsafe-inline'; connect-src 'self'; frame-ancestors 'none'";

async function route(request: NextRequest, nonce: string, csp: string): Promise<NextResponse> {
  // The request that continues to the page carries the policy and the nonce, which is how Next learns the nonce to put on its scripts.
  const forwarded = new Headers(request.headers);
  forwarded.set("Content-Security-Policy", csp);
  forwarded.set("x-nonce", nonce);
  const next = () => NextResponse.next({ request: { headers: forwarded } });
  if (devLoginEnabled()) {
    // The product's own sign-in page instead of the browser prompt: everything but the page, its form handler and
    // static files needs the signed session cookie.
    const { pathname } = request.nextUrl;
    const open = pathname === "/login" || pathname.startsWith("/login-") || pathname.startsWith("/api/dev/") || pathname === "/auth/logout"; // /login-*.jpg: the photographs of the page
    if (open || (await sessionOk(request.cookies.get(DEV_COOKIE)?.value))) return next();
    if (pathname.startsWith("/api/")) return NextResponse.json({ title: "unauthorized", status: 401 }, { status: 401 });
    const url = request.nextUrl.clone();
    url.pathname = "/login";
    url.search = pathname === "/" ? "" : `?returnTo=${encodeURIComponent(pathname)}`;
    return NextResponse.redirect(url);
  }
  if (devAuthEnabled()) {
    // Dev auth signs everybody in. If such an instance is reachable from outside, DEV_BASIC_AUTH puts a password in front.
    const gate = process.env.DEV_BASIC_AUTH;
    if (gate && !basicAuthOk(request.headers.get("authorization"), gate)) {
      return new NextResponse("Authentication required", { status: 401, headers: { "WWW-Authenticate": 'Basic realm="Reliabilix dev", charset="UTF-8"' } });
    }
    return next();
  }
  // The Auth0 SDK builds its own response and does not forward a changed request, so the page could not learn a nonce and its inline
  // scripts would be blocked: this path keeps the legacy policy until it is verified against a real tenant (see ADR-0047).
  const res = await auth0.middleware(request);
  res.headers.set("Content-Security-Policy", LEGACY_CSP);
  return res;
}

export const config = {
  matcher: ["/((?!_next/static|_next/image|favicon.ico|mountains.svg).*)"],
};
