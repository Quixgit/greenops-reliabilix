"use client";

import { ArrowDown, ArrowUp, ChevronsUpDown, Check } from "lucide-react";
import Link from "next/link";
import { useMemo, useState, type CSSProperties, type ReactNode } from "react";
import { cn } from "@/lib/utils";

/** An on/off switch: a pastel track with a thumb that glides. Real `role="switch"`, keyboard and screen-reader friendly. */
export function Switch({ checked, onChange, disabled, label, size = "md" }: {
  checked: boolean; onChange: (v: boolean) => void; disabled?: boolean; label: string; size?: "sm" | "md";
}) {
  const sm = size === "sm";
  return (
    <button type="button" role="switch" aria-checked={checked} aria-label={label} disabled={disabled} onClick={() => onChange(!checked)}
      className={cn("relative inline-flex shrink-0 items-center rounded-full transition-colors duration-300 ease-[var(--ease-out-soft)] focus-visible:outline-2 disabled:cursor-not-allowed disabled:opacity-50",
        sm ? "h-5 w-9" : "h-6 w-11", checked ? "bg-gradient-to-r from-emerald-300 to-emerald-400" : "bg-slate-200")}>
      <span className={cn("inline-block rounded-full bg-white shadow-[0_1px_3px_rgba(15,27,51,.25)] transition-transform duration-300 ease-[var(--ease-out-soft)]",
        sm ? "size-4" : "size-5", checked ? (sm ? "translate-x-[18px]" : "translate-x-[22px]") : "translate-x-0.5")} />
    </button>
  );
}

/** A checkbox with an animated tick; `label` is also what a screen reader says. */
export function Check_({ checked, onChange, disabled, label, children }: {
  checked: boolean; onChange: (v: boolean) => void; disabled?: boolean; label: string; children?: ReactNode;
}) {
  return (
    <label className={cn("inline-flex cursor-pointer items-center gap-2 text-sm", disabled && "cursor-not-allowed opacity-50")}>
      <input type="checkbox" className="peer sr-only" aria-label={label} checked={checked} disabled={disabled} onChange={(e) => onChange(e.target.checked)} />
      <span className="grid size-[18px] place-items-center rounded-[6px] bg-white ring-1 ring-slate-300 transition-all duration-200 peer-checked:bg-emerald-400 peer-checked:ring-emerald-400 peer-focus-visible:outline-2 [&>svg]:scale-0 peer-checked:[&>svg]:scale-100">
        <Check className="size-3 text-white transition-transform duration-200" strokeWidth={3.5} />
      </span>
      {children}
    </label>
  );
}

const TONES = {
  good: "bg-emerald-50 text-emerald-700 ring-emerald-200", warn: "bg-amber-50 text-amber-800 ring-amber-200",
  bad: "bg-rose-50 text-rose-700 ring-rose-200", info: "bg-sky-50 text-sky-700 ring-sky-200",
  neutral: "bg-slate-100 text-slate-600 ring-slate-200", violet: "bg-violet-50 text-violet-700 ring-violet-200",
} as const;
export type PillTone = keyof typeof TONES;

/** A status with a dot; a live status (`pulse`) breathes softly. */
export function Pill({ tone = "neutral", children, pulse }: { tone?: PillTone; children: ReactNode; pulse?: boolean }) {
  const dot = { good: "bg-emerald-500", warn: "bg-amber-500", bad: "bg-rose-500", info: "bg-sky-500", neutral: "bg-slate-400", violet: "bg-violet-500" }[tone];
  return (
    <span className={cn("inline-flex items-center gap-1.5 whitespace-nowrap rounded-full px-2.5 py-0.5 text-xs font-medium ring-1 ring-inset", TONES[tone])}>
      <span className={cn("size-1.5 rounded-full", dot, pulse && "animate-pulse")} />{children}
    </span>
  );
}

/** A horizontal meter (a quota, a share): turns amber then rose as it fills. */
export function Meter({ value, max, label }: { value: number; max: number; label?: string }) {
  const pct = max > 0 ? Math.min(100, (value / max) * 100) : 0;
  const tone = pct >= 90 ? "from-rose-300 to-rose-400" : pct >= 70 ? "from-amber-300 to-amber-400" : "from-emerald-300 to-emerald-400";
  return (
    <div className="flex min-w-24 items-center gap-2" title={label}>
      <div className="h-1.5 flex-1 overflow-hidden rounded-full bg-slate-100"><div className={cn("h-full rounded-full bg-gradient-to-r transition-[width] duration-700", tone)} style={{ width: `${pct}%` }} /></div>
      <span className="w-9 text-right text-xs tabular-nums text-muted">{max > 0 ? `${Math.round(pct)}%` : "—"}</span>
    </div>
  );
}

export type Column<T> = {
  key: string; header: ReactNode; render: (row: T) => ReactNode; align?: "left" | "right" | "center";
  sort?: (a: T, b: T) => number; width?: string; hideBelow?: "md" | "lg";
};

/**
 * The console's table: sticky header, sortable columns, soft row hover, rows that rise in one after another. Rows are
 * links when `href` is given. The same look everywhere so a table is recognisable at a glance.
 */
export function DataTable<T>({ columns, rows, rowKey, href, empty, initialSort, dense, footer }: {
  columns: Column<T>[]; rows: T[]; rowKey: (r: T) => string; href?: (r: T) => string; empty?: ReactNode;
  initialSort?: { key: string; dir: "asc" | "desc" }; dense?: boolean; footer?: ReactNode;
}) {
  const [sort, setSort] = useState(initialSort ?? null);
  const sorted = useMemo(() => {
    const col = columns.find((c) => c.key === sort?.key);
    if (!col?.sort) return rows;
    const out = [...rows].sort(col.sort);
    return sort!.dir === "desc" ? out.reverse() : out;
  }, [rows, columns, sort]);
  const hide = (c: Column<T>) => (c.hideBelow === "md" ? "hidden md:table-cell" : c.hideBelow === "lg" ? "hidden lg:table-cell" : "");
  const al = (a?: string) => (a === "right" ? "text-right" : a === "center" ? "text-center" : "text-left");
  return (
    <div className="overflow-hidden rounded-2xl ring-1 ring-line">
      <div className="thin-scroll max-h-[640px] overflow-auto">
        <table className="w-full min-w-[560px] border-separate border-spacing-0 text-sm">
          <thead className="sticky top-0 z-10">
            <tr>
              {columns.map((c) => {
                const active = sort?.key === c.key;
                return (
                  <th key={c.key} style={{ width: c.width }} aria-sort={active ? (sort!.dir === "asc" ? "ascending" : "descending") : undefined}
                    className={cn("border-b border-line bg-slate-50/95 px-4 py-2.5 text-xs font-semibold uppercase tracking-wide text-slate-500 backdrop-blur", al(c.align), hide(c))}>
                    {c.sort ? (
                      <button type="button" onClick={() => setSort(active && sort!.dir === "asc" ? { key: c.key, dir: "desc" } : { key: c.key, dir: "asc" })}
                        className={cn("inline-flex items-center gap-1 uppercase tracking-wide transition hover:text-ink", active && "text-ink")}>
                        {c.header}
                        {active ? (sort!.dir === "asc" ? <ArrowUp className="size-3" /> : <ArrowDown className="size-3" />) : <ChevronsUpDown className="size-3 opacity-40" />}
                      </button>
                    ) : c.header}
                  </th>
                );
              })}
            </tr>
          </thead>
          <tbody>
            {sorted.length === 0 ? (
              <tr><td colSpan={columns.length} className="px-4 py-10 text-center text-sm text-muted">{empty ?? "Nothing to show."}</td></tr>
            ) : sorted.map((r, i) => (
              <tr key={rowKey(r)} className="rise group transition-colors hover:bg-sky-50/50" style={{ "--i": Math.min(i, 12) } as CSSProperties}>
                {columns.map((c, ci) => (
                  <td key={c.key} className={cn("border-b border-line/60 px-4 align-middle", dense ? "py-2" : "py-3", al(c.align), hide(c))}>
                    {ci === 0 && href ? <Link href={href(r)} className="font-medium text-ink hover:text-primary">{c.render(r)}</Link> : c.render(r)}
                  </td>
                ))}
              </tr>
            ))}
          </tbody>
        </table>
      </div>
      {footer && <div className="border-t border-line bg-slate-50/60 px-4 py-2 text-xs text-muted">{footer}</div>}
    </div>
  );
}
