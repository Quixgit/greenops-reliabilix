"use client";

import { Languages } from "lucide-react";
import { useRouter } from "next/navigation";
import { DropdownPanel, DropdownRadioGroup, DropdownRadioItem, DropdownRoot, DropdownTrigger } from "@/components/ui/dropdown";
import { LOCALE_NAMES, isLocale } from "@/i18n/config";
import { useEnabledLocales, useLocale, useT } from "@/i18n/provider";

/** Chooses the interface language. The choice is a cookie; the page is re-rendered in the new language. */
export function LanguageSwitcher({ className }: { className?: string }) {
  const locale = useLocale();
  const enabled = useEnabledLocales();
  const t = useT();
  const router = useRouter();
  const choose = async (v: string) => {
    if (!isLocale(v) || v === locale) return;
    await fetch("/api/locale", { method: "POST", headers: { "content-type": "application/json" }, body: JSON.stringify({ locale: v }) });
    router.refresh();
  };
  return (
    <DropdownRoot>
      <DropdownTrigger className={className}>
        <Languages className="size-4 text-slate-500" aria-hidden />
        <span className="sr-only">{t("header.language")}: </span>
        <span className="uppercase">{locale}</span>
      </DropdownTrigger>
      <DropdownPanel align="end">
        <DropdownRadioGroup value={locale} onValueChange={choose}>
          {enabled.map((l) => <DropdownRadioItem key={l} value={l}>{LOCALE_NAMES[l]}</DropdownRadioItem>)}
        </DropdownRadioGroup>
      </DropdownPanel>
    </DropdownRoot>
  );
}
