"use client";

import { ChevronLeft, ChevronRight } from "lucide-react";
import { useEffect, useMemo, useState } from "react";
import { useT } from "@/i18n/provider";
import { cn } from "@/lib/utils";

export const PAGE_SIZES = [10, 25, 50] as const;

/** Client-side paging of a list: the slice for the current page and the state the control needs. Resets when the list shrinks. */
export function usePaged<T>(items: T[], initialSize: number = 10) {
  const [page, setPage] = useState(1);
  const [size, setSize] = useState<number>(initialSize);
  const pages = Math.max(1, Math.ceil(items.length / size));
  useEffect(() => { if (page > pages) setPage(pages); }, [page, pages]);
  const slice = useMemo(() => items.slice((page - 1) * size, page * size), [items, page, size]);
  return { slice, page, setPage, size, setSize: (n: number) => { setSize(n); setPage(1); }, pages, total: items.length };
}

/** Page numbers with gaps collapsed: 1 … 4 5 6 … 20. */
function windowed(page: number, pages: number): (number | "gap")[] {
  if (pages <= 7) return Array.from({ length: pages }, (_, i) => i + 1);
  const out: (number | "gap")[] = [1];
  const from = Math.max(2, page - 1), to = Math.min(pages - 1, page + 1);
  if (from > 2) out.push("gap");
  for (let p = from; p <= to; p++) out.push(p);
  if (to < pages - 1) out.push("gap");
  out.push(pages);
  return out;
}

/** The paging bar: "11–20 of 57", a page-size choice, previous and next, and the page numbers. Hidden when one page is enough. */
export function Pagination({ page, pages, total, size, onPage, onSize, className }: {
  page: number; pages: number; total: number; size: number; onPage: (p: number) => void; onSize?: (n: number) => void; className?: string;
}) {
  const t = useT();
  if (total <= PAGE_SIZES[0]) return null;
  const first = (page - 1) * size + 1, last = Math.min(total, page * size);
  const btn = "grid size-8 place-items-center rounded-lg text-sm font-medium transition-all duration-200";
  return (
    <nav aria-label={t("pagination.label")} className={cn("mt-3 flex flex-wrap items-center justify-between gap-3 border-t border-line/70 pt-3", className)}>
      <p className="text-xs tabular-nums text-muted">{t("pagination.range", { from: first, to: last, total })}</p>
      <div className="flex flex-wrap items-center gap-3">
        {onSize && (
          <label className="flex items-center gap-2 text-xs text-muted">
            {t("pagination.per_page")}
            <select value={size} onChange={(e) => onSize(Number(e.target.value))}
              className="rounded-lg border border-line bg-white px-2 py-1 text-xs text-ink outline-none focus:border-emerald-500">
              {PAGE_SIZES.map((n) => <option key={n} value={n}>{n}</option>)}
            </select>
          </label>
        )}
        <div className="flex items-center gap-1">
          <button type="button" aria-label={t("pagination.prev")} disabled={page <= 1} onClick={() => onPage(page - 1)}
            className={cn(btn, "text-slate-600 hover:bg-slate-100 disabled:opacity-35 disabled:hover:bg-transparent")}><ChevronLeft className="size-4" /></button>
          {windowed(page, pages).map((p, i) => p === "gap"
            ? <span key={`g${i}`} className="px-1 text-slate-400" aria-hidden>…</span>
            : (
              <button key={p} type="button" aria-current={p === page ? "page" : undefined} onClick={() => onPage(p)}
                className={cn(btn, "tabular-nums", p === page ? "scale-105 bg-emerald-600 text-white shadow-sm" : "text-slate-600 hover:bg-slate-100")}>{p}</button>
            ))}
          <button type="button" aria-label={t("pagination.next")} disabled={page >= pages} onClick={() => onPage(page + 1)}
            className={cn(btn, "text-slate-600 hover:bg-slate-100 disabled:opacity-35 disabled:hover:bg-transparent")}><ChevronRight className="size-4" /></button>
        </div>
      </div>
    </nav>
  );
}
