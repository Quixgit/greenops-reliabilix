/**
 * A state-changing request that a page of another site made on the person's behalf. The session cookie is SameSite=Lax, which
 * already keeps a browser from sending it, and this is the second wall: a browser says where a request comes from
 * (Sec-Fetch-Site, Origin), and anything but this site is refused. A client that sends neither (a script, curl) is let through:
 * it has no browser to be abused through, and it still needs the cookie. `hosts` are the names this site is known by: the host the
 * request arrived on, the one a proxy forwarded and the public address from the configuration (behind a proxy the first can be internal).
 */
export function crossSite(method: string, headers: Pick<Headers, "get">, hosts: string[]): boolean {
  if (method === "GET" || method === "HEAD") return false;
  const fetchSite = headers.get("sec-fetch-site");
  if (fetchSite && fetchSite !== "same-origin" && fetchSite !== "none") return true;
  const origin = headers.get("origin");
  if (origin) {
    try {
      return !hosts.filter(Boolean).includes(new URL(origin).host);
    } catch {
      return true; // an Origin that is not a URL is not ours
    }
  }
  return false;
}
