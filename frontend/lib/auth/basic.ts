/**
 * Optional password gate for a dev-auth deployment that is reachable from outside (DEV_BASIC_AUTH="user:password").
 * Dev auth signs everybody in as the dev user, so an exposed instance must never run without it.
 */
export function basicAuthOk(header: string | null, expected: string): boolean {
  if (!header?.startsWith("Basic ")) return false;
  let given: string;
  try {
    given = atob(header.slice(6));
  } catch {
    return false;
  }
  // constant-time comparison: do not leak how much of the password matched
  const a = new TextEncoder().encode(given), b = new TextEncoder().encode(expected);
  let diff = a.length ^ b.length;
  for (let i = 0; i < Math.max(a.length, b.length); i++) diff |= (a[i] ?? 0) ^ (b[i] ?? 0);
  return diff === 0;
}
