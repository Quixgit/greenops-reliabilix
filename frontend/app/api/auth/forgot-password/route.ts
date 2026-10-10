import { NextRequest, NextResponse } from "next/server";
import { createLimiter, validEmail } from "@/lib/auth/login";

// Password recovery. The password itself is only ever typed on the identity provider's page; this route asks it to
// send the reset e-mail. The answer is the same whether the address exists or not (no account enumeration).
const allow = createLimiter(5, 60_000);
const OK = () => NextResponse.json({ ok: true }, { headers: { "Cache-Control": "no-store" } });

export async function POST(req: NextRequest) {
  const ip = req.headers.get("x-forwarded-for")?.split(",")[0]?.trim() || "unknown";
  if (!allow(ip)) return NextResponse.json({ title: "Too many requests", status: 429 }, { status: 429, headers: { "Retry-After": "60" } });
  let email: unknown;
  try {
    email = ((await req.json()) as { email?: unknown }).email;
  } catch {
    return NextResponse.json({ title: "invalid request", status: 400 }, { status: 400 });
  }
  if (!validEmail(email)) return NextResponse.json({ title: "invalid e-mail", status: 422 }, { status: 422 });

  const domain = process.env.AUTH0_DOMAIN;
  const clientId = process.env.AUTH0_CLIENT_ID;
  if (domain && clientId) {
    try {
      await fetch(`https://${domain}/dbconnections/change_password`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ client_id: clientId, email: email.trim(), connection: process.env.AUTH0_DB_CONNECTION ?? "Username-Password-Authentication" }),
        signal: AbortSignal.timeout(8000),
        cache: "no-store",
      });
    } catch {
      /* the answer must not reveal anything; a failed call is retried by the person */
    }
  }
  return OK();
}
