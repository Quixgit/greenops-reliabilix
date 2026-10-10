"use client";

import { CornerDownLeft, Search } from "lucide-react";
import { useRouter } from "next/navigation";
import { useEffect, useMemo, useRef, useState } from "react";
import { cn } from "@/lib/utils";
import { ALL_ITEMS } from "./nav";

/** Jump to a console page by name. Opened by the search button in the header; plain navigation, no data. */
export function PageSearch({ open, onClose }: { open: boolean; onClose: () => void }) {
  const router = useRouter();
  const [q, setQ] = useState("");
  const [i, setI] = useState(0);
  const input = useRef<HTMLInputElement>(null);
  const hits = useMemo(() => {
    const needle = q.trim().toLowerCase();
    return needle ? ALL_ITEMS.filter((n) => `${n.label} ${n.hint}`.toLowerCase().includes(needle)) : ALL_ITEMS;
  }, [q]);

  useEffect(() => { if (open) { setQ(""); setI(0); input.current?.focus(); } }, [open]);
  useEffect(() => setI(0), [q]);
  if (!open) return null;

  const go = (href: string) => { onClose(); router.push(href); };
  const onKey = (e: React.KeyboardEvent) => {
    if (e.key === "Escape") onClose();
    else if (e.key === "ArrowDown") { e.preventDefault(); setI((n) => Math.min(hits.length - 1, n + 1)); }
    else if (e.key === "ArrowUp") { e.preventDefault(); setI((n) => Math.max(0, n - 1)); }
    else if (e.key === "Enter" && hits[i]) go(hits[i].href);
  };

  return (
    <div className="fixed inset-0 z-[60] flex items-start justify-center bg-slate-900/25 p-4 pt-[12vh] backdrop-blur-[2px]" onMouseDown={onClose}>
      <div role="dialog" aria-modal="true" aria-label="Go to page" onKeyDown={onKey} onMouseDown={(e) => e.stopPropagation()} className="glass w-full max-w-lg overflow-hidden p-0">
        <div className="flex items-center gap-2 border-b border-line px-4">
          <Search className="size-4 text-muted" aria-hidden />
          <input ref={input} value={q} onChange={(e) => setQ(e.target.value)} placeholder="Go to page…" aria-label="Search pages" role="combobox"
            aria-expanded="true" aria-controls="op-search-list" aria-activedescendant={hits[i] ? `op-search-${i}` : undefined}
            className="h-12 w-full bg-transparent text-sm outline-none placeholder:text-muted" />
        </div>
        <ul id="op-search-list" role="listbox" className="max-h-80 overflow-y-auto p-1.5">
          {hits.length === 0 && <li className="px-3 py-6 text-center text-sm text-muted">No page matches “{q}”.</li>}
          {hits.map((n, idx) => (
            <li key={n.href} id={`op-search-${idx}`} role="option" aria-selected={idx === i} onMouseEnter={() => setI(idx)} onClick={() => go(n.href)}
              className={cn("flex cursor-pointer items-center gap-3 rounded-xl px-3 py-2 text-sm", idx === i ? "bg-white shadow-sm ring-1 ring-line" : "")}>
              <n.icon className="size-4 text-primary" aria-hidden />
              <span className="font-medium">{n.label}</span>
              <span className="truncate text-xs text-muted">{n.hint}</span>
              {n.planned && <span className="ml-auto rounded-full bg-violet-50 px-2 py-0.5 text-[11px] font-medium text-violet-700 ring-1 ring-violet-200">planned</span>}
              {idx === i && !n.planned && <CornerDownLeft className="ml-auto size-3.5 text-muted" aria-hidden />}
            </li>
          ))}
        </ul>
      </div>
    </div>
  );
}
