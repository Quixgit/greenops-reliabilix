"use client";

import { useT } from "@/i18n/provider";
import { useUi } from "@/lib/store";
import { cn } from "@/lib/utils";

/** Two chevrons that lean the way the menu will move; on hover they slide that way, one after the other. */
export function CollapseToggle({ className }: { className?: string }) {
  const { navCollapsed, setNavCollapsed } = useUi();
  const t = useT();
  const label = navCollapsed ? t("nav.expand", undefined, "Expand the menu") : t("nav.collapse", undefined, "Collapse the menu");
  return (
    <button type="button" onClick={() => setNavCollapsed(!navCollapsed)} aria-label={label} title={label} aria-expanded={!navCollapsed}
      className={cn("chev-toggle group grid size-8 place-items-center rounded-full text-slate-400 transition-colors hover:bg-white/10 hover:text-white", className)}>
      <svg viewBox="0 0 24 24" className={cn("size-5 transition-transform duration-500", navCollapsed && "rotate-180")} fill="none" stroke="currentColor" strokeWidth="2.2" strokeLinecap="round" strokeLinejoin="round" aria-hidden>
        <path className="chev chev-a" d="M13 6l-6 6 6 6" />
        <path className="chev chev-b" d="M19 6l-6 6 6 6" />
      </svg>
    </button>
  );
}
