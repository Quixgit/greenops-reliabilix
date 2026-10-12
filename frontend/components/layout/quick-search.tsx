"use client";

import { IconSearch } from "@/components/ui/icons";
import { useEffect, useMemo, useRef, useState } from "react";
import { useT } from "@/i18n/provider";
import { cn } from "@/lib/utils";

export type QuickItem = { label: string; hint: string; run: () => void };

/**
 * The header's search: type to jump to a page or an item. ⌘K / Ctrl+K focuses it; arrows move, Enter opens the match.
 * The client workspace and the operator console use this one component, so the two headers behave the same.
 */
export function QuickSearchBox({ all }: { all: QuickItem[] }) {
  const t = useT();
  const [q, setQ] = useState("");
  const [open, setOpen] = useState(false);
  const [cur, setCur] = useState(0);
  const box = useRef<HTMLInputElement>(null);
  useEffect(() => {
    const k = (e: KeyboardEvent) => { if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === "k") { e.preventDefault(); box.current?.focus(); setOpen(true); } };
    window.addEventListener("keydown", k);
    return () => window.removeEventListener("keydown", k);
  }, []);
  const items = useMemo(() => {
    const n = q.trim().toLowerCase();
    return (n ? all.filter((i) => i.label.toLowerCase().includes(n)) : all.slice(0, 7)).slice(0, 8);
  }, [q, all]);
  const go = (i: number) => { items[i]?.run(); setOpen(false); setQ(""); box.current?.blur(); };
  return (
    <div className="relative hidden md:block">
      <IconSearch className="pointer-events-none absolute left-3.5 top-1/2 size-[18px] -translate-y-1/2 text-slate-400" />
      <input ref={box} value={q} placeholder={t("header.search")} aria-label={t("header.search")} role="combobox" aria-expanded={open} aria-controls="quick-search"
        onChange={(e) => { setQ(e.target.value); setCur(0); setOpen(true); }} onFocus={() => setOpen(true)} onBlur={() => window.setTimeout(() => setOpen(false), 120)}
        onKeyDown={(e) => {
          if (e.key === "ArrowDown") { e.preventDefault(); setCur((c) => Math.min(items.length - 1, c + 1)); }
          else if (e.key === "ArrowUp") { e.preventDefault(); setCur((c) => Math.max(0, c - 1)); }
          else if (e.key === "Enter") { e.preventDefault(); go(cur); }
          else if (e.key === "Escape") { setOpen(false); box.current?.blur(); }
        }}
        className="h-10 w-40 rounded-xl border border-line/80 bg-white pl-11 pr-4 text-sm text-ink shadow-[0_1px_2px_rgba(16,24,40,0.04)] placeholder:text-slate-400 lg:w-56 xl:w-72 xl:pr-14" />
      <kbd className="pointer-events-none absolute right-3 hidden xl:block top-1/2 -translate-y-1/2 rounded-md bg-slate-100 px-1.5 py-0.5 text-[10px] font-medium text-slate-500">⌘K</kbd>
      {open && items.length > 0 && (
        <ul id="quick-search" role="listbox" className="info-pop absolute right-0 top-12 z-50 w-80 overflow-hidden rounded-2xl border border-line bg-white p-1.5 shadow-[0_18px_44px_rgba(31,52,110,0.16)]">
          {items.map((it, i) => (
            <li key={`${it.hint}-${it.label}`} role="option" aria-selected={i === cur}>
              <button type="button" onMouseDown={(e) => e.preventDefault()} onClick={() => go(i)} onMouseEnter={() => setCur(i)}
                className={cn("flex w-full items-center justify-between gap-3 rounded-lg px-3 py-2 text-left text-sm transition-colors", i === cur ? "bg-leaf-soft/70" : "")}>
                <span className="truncate font-medium">{it.label}</span><span className="shrink-0 text-xs text-muted">{it.hint}</span>
              </button>
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}
