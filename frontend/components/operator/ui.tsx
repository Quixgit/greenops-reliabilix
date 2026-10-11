"use client";

import * as DM from "@radix-ui/react-dropdown-menu";
import type { AnyIcon } from "@/components/ui/icons";
import { IconArrowDown as ArrowDown, IconArrowUp as ArrowUp, IconChevronsUpDown as ChevronsUpDown, IconCheck as Check, IconChevronDown as ChevronDown, IconSearch as Search } from "@/components/ui/icons";
import { DropdownPanel, DropdownRadioGroup, DropdownRadioItem, DropdownRoot } from "@/components/ui/dropdown";
import { PAGE_SIZES, Pagination, usePaged } from "@/components/ui/pagination";
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
        sm ? "h-5 w-9" : "h-6 w-11", checked ? "bg-leaf-deep/75" : "bg-slate-200")}>
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
      <span className="grid size-[18px] place-items-center rounded-[6px] bg-white ring-1 ring-slate-300 transition-all duration-200 peer-checked:bg-leaf-deep/80 peer-checked:ring-leaf-deep/80 peer-focus-visible:outline-2 [&>svg]:scale-0 peer-checked:[&>svg]:scale-100">
        <Check className="size-3 text-white transition-transform duration-200" strokeWidth={3.5} />
      </span>
      {children}
    </label>
  );
}

const TONES = {
  good: "bg-leaf-soft text-leaf-deep ring-leaf-soft", warn: "bg-amber-50 text-[#a8761f] ring-amber-200",
  bad: "bg-rose-50 text-[#b9566a] ring-rose-200", info: "bg-sky-50 text-[#3f78b8] ring-sky-200",
  neutral: "bg-slate-100 text-slate-600 ring-slate-200", violet: "bg-violet-50 text-violet-700 ring-violet-200",
} as const;
export type PillTone = keyof typeof TONES;

/** A status with a dot; a live status (`pulse`) breathes softly. */
export function Pill({ tone = "neutral", children, pulse }: { tone?: PillTone; children: ReactNode; pulse?: boolean }) {
  const dot = { good: "bg-leaf", warn: "bg-pastel-amber", bad: "bg-pastel-rose", info: "bg-pastel-blue", neutral: "bg-slate-400", violet: "bg-pastel-violet" }[tone];
  return (
    <span className={cn("inline-flex items-center gap-1.5 whitespace-nowrap rounded-full px-2.5 py-0.5 text-xs font-medium ring-1 ring-inset", TONES[tone])}>
      <span className={cn("size-1.5 rounded-full", dot, pulse && "animate-pulse")} />{children}
    </span>
  );
}

/** A horizontal meter (a quota, a share): turns amber then rose as it fills. */
export function Meter({ value, max, label }: { value: number; max: number; label?: string }) {
  const pct = max > 0 ? Math.min(100, (value / max) * 100) : 0;
  const tone = pct >= 90 ? "from-pastel-rose to-pastel-rose" : pct >= 70 ? "from-pastel-amber to-pastel-amber" : "from-leaf to-leaf";
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
 * The console's table, the platform's way: one card, soft header, sortable columns, rows that rise in one after another,
 * and the client's paging bar when there is more than a page. It never scrolls vertically (long lists are paged); it
 * scrolls sideways only when the screen is narrower than its columns. Rows are links when `href` is given.
 * `flush` drops the card when the table already sits in one.
 */
export function DataTable<T>({ columns, rows, rowKey, href, empty, initialSort, dense, footer, pageSize = 15, flush }: {
  columns: Column<T>[]; rows: T[]; rowKey: (r: T) => string; href?: (r: T) => string; empty?: ReactNode;
  initialSort?: { key: string; dir: "asc" | "desc" }; dense?: boolean; footer?: ReactNode; pageSize?: number; flush?: boolean;
}) {
  const [sort, setSort] = useState(initialSort ?? null);
  const sorted = useMemo(() => {
    const col = columns.find((c) => c.key === sort?.key);
    if (!col?.sort) return rows;
    const out = [...rows].sort(col.sort);
    return sort!.dir === "desc" ? out.reverse() : out;
  }, [rows, columns, sort]);
  const paged = usePaged(sorted, pageSize);
  const hide = (c: Column<T>) => (c.hideBelow === "md" ? "hidden md:table-cell" : c.hideBelow === "lg" ? "hidden lg:table-cell" : "");
  const al = (a?: string) => (a === "right" ? "text-right" : a === "center" ? "text-center" : "text-left");
  return (
    <div className={cn(!flush && "glass overflow-hidden")}>
      <div className="thin-scroll overflow-x-auto overflow-y-hidden">
        <table className="w-full min-w-[560px] border-separate border-spacing-0 text-sm">
          <thead>
            <tr>
              {columns.map((c) => {
                const active = sort?.key === c.key;
                return (
                  <th key={c.key} style={{ width: c.width }} aria-sort={active ? (sort!.dir === "asc" ? "ascending" : "descending") : undefined}
                    className={cn("border-b border-line bg-slate-50/70 px-4 py-2.5 text-xs font-semibold uppercase tracking-wide text-slate-500", al(c.align), hide(c))}>
                    {c.sort ? (
                      <button type="button" onClick={() => { setSort(active && sort!.dir === "asc" ? { key: c.key, dir: "desc" } : { key: c.key, dir: "asc" }); paged.setPage(1); }}
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
            ) : paged.slice.map((r, i) => (
              <tr key={rowKey(r)} className="rise group transition-colors hover:bg-slate-50/80" style={{ "--i": Math.min(i, 12) } as CSSProperties}>
                {columns.map((c, ci) => (
                  <td key={c.key} className={cn("border-b border-line/60 px-4 align-middle group-last:border-b-0", dense ? "py-2" : "py-3", al(c.align), hide(c))}>
                    {ci === 0 && href ? <Link href={href(r)} className="font-medium text-ink hover:text-primary">{c.render(r)}</Link> : c.render(r)}
                  </td>
                ))}
              </tr>
            ))}
          </tbody>
        </table>
      </div>
      {(footer || sorted.length > PAGE_SIZES[0]) && (
        <div className="border-t border-line bg-slate-50/50 px-4 py-2 text-xs text-muted">
          {footer}
          <Pagination page={paged.page} pages={paged.pages} total={paged.total} size={paged.size} onPage={paged.setPage} onSize={paged.setSize} className="mt-0 border-t-0 pt-0" />
        </div>
      )}
    </div>
  );
}

/**
 * A list of rows in the platform's list style (like the client's reports and recommendations): a large drawn icon, a title
 * with a line under it, the state and the actions on the right, a soft hover, paged. Each row is yours to draw.
 */
export function RowList<T>({ rows, rowKey, row, empty, pageSize = 10 }: { rows: T[]; rowKey: (r: T) => string; row: (r: T, i: number) => ReactNode; empty?: ReactNode; pageSize?: number }) {
  const paged = usePaged(rows, pageSize);
  if (rows.length === 0) return <div className="glass px-6 py-12">{empty ?? <EmptyPanel title="Nothing to show" />}</div>;
  return (
    <div className="glass p-2">
      <ul className="divide-y divide-line/60">
        {paged.slice.map((r, i) => <li key={rowKey(r)} className="rise" style={{ "--i": Math.min(i, 8) } as CSSProperties}>{row(r, i)}</li>)}
      </ul>
      <Pagination page={paged.page} pages={paged.pages} total={paged.total} size={paged.size} onPage={paged.setPage} onSize={paged.setSize} className="mx-3 mb-2" />
    </div>
  );
}

/** What a list says when it is empty: a calm icon and a line, in the tone of good news or of "nothing yet". */
export function EmptyPanel({ title, text, icon: Icon = Check, tone = "good" }: { title: string; text?: string; icon?: AnyIcon; tone?: "good" | "quiet" }) {
  return (
    <div className="rise flex flex-col items-center gap-2 text-center">
      <span className={cn("grid size-14 place-items-center rounded-full", tone === "good" ? "bg-leaf-soft text-leaf-deep" : "bg-slate-100 text-slate-400")}><Icon className="size-7" strokeWidth={1.5} /></span>
      <p className="font-semibold">{title}</p>
      {text && <p className="max-w-md text-sm text-muted">{text}</p>}
    </div>
  );
}

// ---- Controls shared by every console page: one look, soft tones, gentle motion ---------------------------------------

const control = "h-10 rounded-xl border border-line/80 bg-white text-sm text-ink shadow-[0_1px_2px_rgba(16,24,40,0.04)] outline-none transition-all duration-200 hover:border-slate-300 focus-visible:border-leaf focus-visible:shadow-[0_0_0_3px_rgba(111,195,168,0.22)] disabled:cursor-not-allowed disabled:opacity-50";
/** The class of a text input, so a field in a form and a field in a toolbar are the same thing. */
export const inputCls = cn(control, "block w-full px-3.5 placeholder:text-slate-400 disabled:bg-slate-50");

/** A drop-down list in the platform's style (replaces the browser's own select, which looked foreign and could not animate). */
export function Select({ value, onChange, options, label, disabled, className, placeholder }: {
  value: string; onChange: (v: string) => void; options: { value: string; label: string }[]; label: string; disabled?: boolean; className?: string; placeholder?: string;
}) {
  const current = options.find((o) => o.value === value);
  return (
    <DropdownRoot>
      <DM.Trigger aria-label={label} disabled={disabled} className={cn(control, "group inline-flex items-center justify-between gap-2 px-3.5 font-medium data-[state=open]:border-leaf data-[state=open]:shadow-[0_0_0_3px_rgba(111,195,168,0.22)]", className)}>
        <span className="truncate">{current?.label ?? placeholder ?? label}</span>
        <ChevronDown className="size-4 shrink-0 text-slate-400 transition-transform duration-300 group-data-[state=open]:rotate-180" aria-hidden />
      </DM.Trigger>
      <DropdownPanel>
        <DropdownRadioGroup value={value} onValueChange={onChange}>
          {options.map((o) => <DropdownRadioItem key={o.value} value={o.value}>{o.label}</DropdownRadioItem>)}
        </DropdownRadioGroup>
      </DropdownPanel>
    </DropdownRoot>
  );
}

/** A search field with the magnifier inside; Escape clears it. */
export function SearchBox({ value, onChange, placeholder, label, className }: { value: string; onChange: (v: string) => void; placeholder: string; label?: string; className?: string }) {
  return (
    <div className={cn("relative", className)}>
      <Search className="pointer-events-none absolute left-3.5 top-1/2 size-[18px] -translate-y-1/2 text-slate-400" aria-hidden />
      <input value={value} onChange={(e) => onChange(e.target.value)} onKeyDown={(e) => { if (e.key === "Escape") onChange(""); }}
        placeholder={placeholder} aria-label={label ?? placeholder} className={cn(inputCls, "pl-11")} />
    </div>
  );
}

/** Filters as one segmented control; the lit segment glides softly. `count` shows how many each would show. */
export function Segmented({ value, onChange, options, label }: {
  value: string; onChange: (v: string) => void; label: string; options: { value: string; label: string; count?: number }[];
}) {
  return (
    <div role="tablist" aria-label={label} className="inline-flex h-10 items-center rounded-xl border border-line/80 bg-white p-1 shadow-[0_1px_2px_rgba(16,24,40,0.04)]">
      {options.map((o) => (
        <button key={o.value} role="tab" type="button" aria-selected={value === o.value} onClick={() => onChange(o.value)}
          className={cn("inline-flex items-center gap-1.5 rounded-lg px-3 py-1 text-[13px] font-medium transition-all duration-200", value === o.value ? "bg-leaf-soft text-leaf-deep shadow-sm" : "text-slate-500 hover:text-ink")}>
          {o.label}
          {o.count != null && <span className={cn("rounded-full px-1.5 text-[11px] tabular-nums", value === o.value ? "bg-white/70" : "bg-slate-100")}>{o.count}</span>}
        </button>
      ))}
    </div>
  );
}

/** The primary and quiet buttons of the console: flat green, and white with a ring. */
export function Btn({ tone = "primary", size = "md", className, ...p }: React.ButtonHTMLAttributes<HTMLButtonElement> & { tone?: "primary" | "quiet" | "danger"; size?: "sm" | "md" }) {
  return (
    <button type="button" {...p}
      className={cn("inline-flex items-center justify-center gap-2 rounded-xl font-semibold outline-none transition focus-visible:ring-2 focus-visible:ring-leaf/50 disabled:cursor-not-allowed disabled:opacity-50",
        size === "sm" ? "h-8 px-3 text-xs" : "h-10 px-4 text-sm",
        tone === "primary" && "btn-leaf", tone === "quiet" && "bg-white text-ink ring-1 ring-line hover:bg-slate-50",
        tone === "danger" && "bg-rose-50 text-[#c2566a] ring-1 ring-rose-200 hover:bg-rose-100", className)} />
  );
}

/** A soft tinted initial for a name: the same name always gets the same tone, so a list is easy to scan. */
export function Avatar({ name, size = "md" }: { name: string; size?: "sm" | "md" | "lg" }) {
  const tones = ["bg-leaf-soft text-leaf-deep", "bg-sky-50 text-sky-700", "bg-violet-50 text-violet-700", "bg-amber-50 text-[#a8761f]", "bg-rose-50 text-[#c2566a]"];
  let h = 0;
  for (const c of name) h = (h * 31 + c.charCodeAt(0)) >>> 0;
  return <span aria-hidden className={cn("inline-grid shrink-0 place-items-center rounded-xl font-bold", tones[h % tones.length], size === "sm" ? "size-8 text-xs" : size === "lg" ? "size-12 text-lg" : "size-9 text-sm")}>{(name.match(/[\p{L}\p{N}]/u)?.[0] ?? "?").toUpperCase()}</span>;
}
