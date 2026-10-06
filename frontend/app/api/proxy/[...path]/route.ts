import { NextRequest, NextResponse } from "next/server";
import { getApiToken } from "@/lib/auth/session";

// Browser -> this route -> Go API. The access token is attached server-side and never reaches
// client JavaScript. Only the API's /api/v1 namespace is reachable.
const API_URL = process.env.API_URL ?? "http://localhost:8080";
const ALLOWED = new Set(["GET", "POST"]);

async function handle(req: NextRequest, ctx: { params: Promise<{ path: string[] }> }) {
  if (!ALLOWED.has(req.method)) return NextResponse.json({ title: "method not allowed", status: 405 }, { status: 405 });
  const token = await getApiToken();
  if (!token) return NextResponse.json({ title: "unauthorized", status: 401 }, { status: 401 });

  const { path } = await ctx.params;
  if (path.some((s) => s === ".." || s === "." || s.includes("/"))) {
    return NextResponse.json({ title: "bad request", status: 400 }, { status: 400 });
  }
  const url = `${API_URL}/api/v1/${path.map(encodeURIComponent).join("/")}${req.nextUrl.search}`;
  const upstream = await fetch(url, {
    method: req.method,
    headers: { Authorization: `Bearer ${token}`, "Content-Type": "application/json" },
    body: req.method === "POST" ? await req.text() : undefined,
    cache: "no-store",
  });
  return new NextResponse(upstream.body, {
    status: upstream.status,
    headers: { "Content-Type": upstream.headers.get("Content-Type") ?? "application/json", "Cache-Control": "no-store" },
  });
}

export { handle as GET, handle as POST };
