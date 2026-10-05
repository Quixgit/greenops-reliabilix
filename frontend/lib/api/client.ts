import type { z } from "zod";

export class ApiError extends Error {
  constructor(public status: number, message: string) {
    super(message);
  }
}

/** Calls the Go API through the same-origin proxy and validates the response with zod. */
export async function api<S extends z.ZodTypeAny>(
  path: string,
  schema: S,
  params?: Record<string, string | undefined>,
  init?: RequestInit,
): Promise<z.output<S>> {
  const qs = new URLSearchParams();
  for (const [k, v] of Object.entries(params ?? {})) if (v) qs.set(k, v);
  const res = await fetch(`/api/proxy/${path}${qs.size ? `?${qs}` : ""}`, { ...init, headers: { "Content-Type": "application/json" } });
  if (!res.ok) {
    let title = res.statusText;
    try { title = ((await res.json()) as { title?: string }).title ?? title; } catch { /* non-JSON error body */ }
    throw new ApiError(res.status, title);
  }
  return schema.parse(await res.json());
}
