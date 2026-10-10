import { basicAuthOk } from "./basic";
import { devAuthEnabled } from "./dev";

/**
 * A real sign-in page for a development deployment that is reachable from outside (DEV_AUTH + DEV_BASIC_AUTH +
 * DEV_LOGIN_PAGE=true). Instead of the browser's password prompt, people see the product's sign-in page, type the
 * development credentials (DEV_BASIC_AUTH="user:password") and get a signed, http-only cookie. Never used with Auth0.
 */
export const DEV_COOKIE = "rlx_dev_session";

export function devLoginEnabled(): boolean {
  return devAuthEnabled() && !!process.env.DEV_BASIC_AUTH && process.env.DEV_LOGIN_PAGE === "true";
}

async function hmacHex(secret: string, message: string): Promise<string> {
  const key = await crypto.subtle.importKey("raw", new TextEncoder().encode(secret), { name: "HMAC", hash: "SHA-256" }, false, ["sign"]);
  const sig = new Uint8Array(await crypto.subtle.sign("HMAC", key, new TextEncoder().encode(message)));
  return Array.from(sig, (b) => b.toString(16).padStart(2, "0")).join("");
}

/** The cookie value of a valid session: a MAC under the configured password, so it cannot be forged without it. */
export async function sessionValue(): Promise<string> {
  return hmacHex(process.env.DEV_BASIC_AUTH ?? "", "reliabilix-dev-session-v1");
}

export async function sessionOk(value: string | undefined): Promise<boolean> {
  if (!value || !devLoginEnabled()) return false;
  const want = await sessionValue();
  if (value.length !== want.length) return false;
  let diff = 0;
  for (let i = 0; i < want.length; i++) diff |= value.charCodeAt(i) ^ want.charCodeAt(i);
  return diff === 0;
}

/** user and password as typed in the form against DEV_BASIC_AUTH, compared in constant time. */
export function credentialsOk(user: string, password: string): boolean {
  const expected = process.env.DEV_BASIC_AUTH ?? "";
  return !!expected && basicAuthOk(`Basic ${btoa(`${user}:${password}`)}`, expected);
}
