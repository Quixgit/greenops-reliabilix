import "server-only";
import { cookies, headers } from "next/headers";
import { DEFAULT_LOCALE, LOCALES, LOCALE_COOKIE, isLocale, negotiate, type Locale } from "./config";
import type { Messages } from "./translate";

export type LocaleConfig = { default: Locale; enabled: Locale[] };

/** The platform's language settings (public endpoint). On any failure every built-in language is on offer. */
export async function getLocaleConfig(): Promise<LocaleConfig> {
  const fallback: LocaleConfig = { default: DEFAULT_LOCALE, enabled: [...LOCALES] };
  const base = process.env.API_URL;
  if (!base) return fallback;
  try {
    const res = await fetch(`${base}/api/v1/locales`, { next: { revalidate: 60 }, signal: AbortSignal.timeout(2000) });
    if (!res.ok) return fallback;
    const j = (await res.json()) as { default?: unknown; enabled?: unknown };
    const enabled = Array.isArray(j.enabled) ? j.enabled.filter(isLocale) : [];
    const def = isLocale(j.default) ? j.default : DEFAULT_LOCALE;
    return { default: enabled.includes(def) || enabled.length === 0 ? def : enabled[0], enabled: enabled.length ? enabled : [def] };
  } catch {
    return fallback;
  }
}

/** The language of this request: the person's choice (cookie), else the browser's, else the platform default; always one on offer. */
export async function getLocale(config?: LocaleConfig): Promise<Locale> {
  const cfg = config ?? (await getLocaleConfig());
  const chosen = (await cookies()).get(LOCALE_COOKIE)?.value;
  if (isLocale(chosen) && cfg.enabled.includes(chosen)) return chosen;
  const asked = negotiate((await headers()).get("accept-language"), cfg.enabled);
  return asked ?? cfg.default;
}

const loaders: Record<Locale, () => Promise<{ default: Messages }>> = {
  en: () => import("./messages/en.json"),
  ru: () => import("./messages/ru.json"),
};

export async function getMessages(locale: Locale): Promise<{ messages: Messages; fallback: Messages }> {
  const [messages, fallback] = await Promise.all([loaders[locale](), loaders[DEFAULT_LOCALE]()]);
  return { messages: messages.default, fallback: fallback.default };
}
