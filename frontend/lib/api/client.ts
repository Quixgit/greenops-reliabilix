import type { z } from "zod";

export class ApiError extends Error {
  /** For validation failures (422): one message per rejected field, for example per setting key. */
  constructor(public status: number, message: string, public fields: Record<string, string> = {}) {
    super(message);
  }
}

async function fail(res: Response): Promise<never> {
  let title = res.statusText;
  let fields: Record<string, string> = {};
  try {
    const body = (await res.json()) as { title?: string; detail?: string; errors?: Record<string, string> };
    title = body.detail || body.title || title;
    fields = body.errors ?? {};
  } catch { /* non-JSON error body */ }
  throw new ApiError(res.status, title, fields);
}

/** For calls that answer 204 No Content (deletes). */
export async function apiVoid(path: string, init?: RequestInit): Promise<void> {
  const res = await fetch(`/api/proxy/${path}`, { ...init, headers: { "Content-Type": "application/json" } });
  if (!res.ok) await fail(res);
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
  if (!res.ok) await fail(res);
  return schema.parse(await res.json());
}
