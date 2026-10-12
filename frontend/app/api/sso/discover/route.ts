import { NextRequest, NextResponse } from "next/server";
import { createLimiter, validEmail } from "@/lib/auth/login";

// Where does an address sign in? Public (the sign-in page asks before anyone is signed in); it answers with the name of a company connection or
// nothing, and is rate limited per address of the caller. The API holds the truth: a domain an organization has verified.
const allow = createLimiter(30, 60_000);

export async function GET(req: NextRequest) {
  const ip = req.headers.get("x-forwarded-for")?.split(",")[0]?.trim() || "unknown";
  if (!allow(ip)) return NextResponse.json({ connection: "" }, { status: 429 });
  const email = req.nextUrl.searchParams.get("email") ?? "";
  const base = process.env.API_URL;
  if (!base || !validEmail(email)) return NextResponse.json({ connection: "" });
  try {
    const res = await fetch(`${base}/api/v1/sso/discover?email=${encodeURIComponent(email.trim())}`, { cache: "no-store", signal: AbortSignal.timeout(3000) });
    if (!res.ok) return NextResponse.json({ connection: "" });
    const j = (await res.json()) as { connection?: unknown };
    const c = typeof j.connection === "string" && /^[A-Za-z0-9][A-Za-z0-9_.-]{0,99}$/.test(j.connection) ? j.connection : "";
    return NextResponse.json({ connection: c }, { headers: { "Cache-Control": "no-store" } });
  } catch {
    return NextResponse.json({ connection: "" }); // the ordinary sign-in still works
  }
}
