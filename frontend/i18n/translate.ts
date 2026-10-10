import type { Locale } from "./config";

export type Messages = Record<string, string>;
export type Params = Record<string, string | number>;

/**
 * Looks a message up (the language, then English, then the caller's fallback, then the key itself: a visible, safe
 * failure) and fills {name} placeholders. Plural forms are separate keys: "key.one", "key.few", "key.many", "key.other",
 * chosen with Intl.PluralRules on the `count` parameter.
 */
export function translate(messages: Messages, fallbackMessages: Messages, locale: Locale, key: string, params?: Params, fallback?: string): string {
  let k = key;
  if (params && typeof params.count === "number") {
    const form = new Intl.PluralRules(locale).select(params.count);
    for (const candidate of [`${key}.${form}`, `${key}.other`]) {
      if (candidate in messages || candidate in fallbackMessages) { k = candidate; break; }
    }
  }
  const raw = messages[k] ?? fallbackMessages[k] ?? fallback ?? key;
  if (!params) return raw;
  return raw.replace(/\{([a-z_][a-z0-9_]*)\}/gi, (m, name: string) => (name in params ? String(params[name]) : m));
}

/** The placeholders of a message, sorted: used by the catalog parity test. */
export function placeholders(message: string): string[] {
  return Array.from(message.matchAll(/\{([a-z_][a-z0-9_]*)\}/gi), (m) => m[1]).sort();
}
