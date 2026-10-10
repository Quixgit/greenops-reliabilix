import { NextRequest, NextResponse } from "next/server";
import { LOCALE_COOKIE, isLocale } from "@/i18n/config";

// Remembers the person's language in a cookie. Only a supported language is accepted; nothing else is stored.
export async function POST(req: NextRequest) {
  const body = (await req.json().catch(() => null)) as { locale?: unknown } | null;
  if (!body || !isLocale(body.locale)) return NextResponse.json({ error: "unsupported language" }, { status: 400 });
  const res = NextResponse.json({ locale: body.locale });
  res.cookies.set(LOCALE_COOKIE, body.locale, { path: "/", maxAge: 365 * 24 * 3600, sameSite: "lax", httpOnly: false, secure: req.nextUrl.protocol === "https:" });
  return res;
}
