"use client";

import { createContext, useCallback, useContext, useMemo, type ReactNode } from "react";
import { setFormatLocale } from "@/lib/formatters";
import type { Locale } from "./config";
import { translate, type Messages, type Params } from "./translate";

type Ctx = { locale: Locale; messages: Messages; fallback: Messages; enabled: Locale[] };
const I18nContext = createContext<Ctx>({ locale: "en", messages: {}, fallback: {}, enabled: ["en"] });

export function I18nProvider({ locale, messages, fallback, enabled, children }: Ctx & { children: ReactNode }) {
  setFormatLocale(locale); // numbers, currencies and dates follow the language everywhere, including old call sites
  const value = useMemo(() => ({ locale, messages, fallback, enabled }), [locale, messages, fallback, enabled]);
  return <I18nContext.Provider value={value}>{children}</I18nContext.Provider>;
}

/** `t("nav.overview")`, `t("greeting", { name })`, `t("items", { count })`, `t("key", undefined, "English fallback")`. */
export function useT() {
  const { locale, messages, fallback } = useContext(I18nContext);
  return useCallback((key: string, params?: Params, fallbackText?: string) => translate(messages, fallback, locale, key, params, fallbackText), [locale, messages, fallback]);
}

export const useLocale = (): Locale => useContext(I18nContext).locale;
/** The languages the platform offers (platform setting), for the switcher. */
export const useEnabledLocales = (): Locale[] => useContext(I18nContext).enabled;
