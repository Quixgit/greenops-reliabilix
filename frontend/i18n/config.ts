/** Languages of the interface. A language is added here, as a catalog in ./messages and on the server (see ADR-0036). */
export const LOCALES = ["en", "ru"] as const;
export type Locale = (typeof LOCALES)[number];
export const DEFAULT_LOCALE: Locale = "en";
export const LOCALE_COOKIE = "rlx_locale";
export const LOCALE_NAMES: Record<Locale, string> = { en: "English", ru: "Русский" };

export const isLocale = (v: unknown): v is Locale => typeof v === "string" && (LOCALES as readonly string[]).includes(v);

/** Best offered language for an Accept-Language value ("ru-RU,ru;q=0.9,en;q=0.8"); only the primary subtag is compared. undefined: none matches. */
export function negotiate(header: string | null | undefined, offered: readonly Locale[] = LOCALES): Locale | undefined {
  const found = (header ?? "").split(",").map((part, pos) => {
    const [tag, ...params] = part.trim().split(";");
    const q = Number(params.map((p) => p.trim()).find((p) => p.startsWith("q="))?.slice(2) ?? 1);
    return { lang: tag.split(/[-_]/)[0].toLowerCase(), q: Number.isFinite(q) ? q : 1, pos };
  }).filter((c) => (offered as readonly string[]).includes(c.lang) && c.q > 0);
  found.sort((a, b) => b.q - a.q || a.pos - b.pos);
  return found[0]?.lang as Locale | undefined;
}
