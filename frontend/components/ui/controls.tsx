"use client";

import { useId, type ReactNode } from "react";
import { cn } from "@/lib/utils";

export interface SegOption<T extends string> { value: T; label: ReactNode; title?: string; disabled?: boolean }

/**
 * Segmented switch: options of equal width and one thumb that slides under the chosen one. It is a radio group (arrow keys move between the
 * options), so a keyboard and a screen reader get the same control as a pointer.
 */
export function Segmented<T extends string>({ value, onChange, options, label, size = "md", className }: {
  value: T; onChange: (v: T) => void; options: SegOption<T>[]; label: string; size?: "sm" | "md"; className?: string;
}) {
  const i = Math.max(0, options.findIndex((o) => o.value === value));
  const n = options.length;
  const move = (d: number) => {
    for (let k = 1; k <= n; k++) {
      const o = options[(i + d * k + n * k) % n];
      if (!o.disabled) { onChange(o.value); return; }
    }
  };
  return (
    <div role="radiogroup" aria-label={label}
      onKeyDown={(e) => { if (e.key === "ArrowRight" || e.key === "ArrowDown") { e.preventDefault(); move(1); } else if (e.key === "ArrowLeft" || e.key === "ArrowUp") { e.preventDefault(); move(-1); } }}
      className={cn("relative inline-grid auto-cols-fr grid-flow-col rounded-xl bg-slate-100/80 p-0.5 ring-1 ring-line/60", className)}>
      <span aria-hidden className="absolute inset-y-0.5 left-0.5 rounded-[10px] bg-white shadow-sm ring-1 ring-black/5 transition-transform duration-300 ease-[var(--ease-out-soft)]"
        style={{ width: `calc((100% - 4px) / ${n})`, transform: `translateX(${i * 100}%)` }} />
      {options.map((o) => (
        <button key={o.value} type="button" role="radio" aria-checked={o.value === value} tabIndex={o.value === value ? 0 : -1} disabled={o.disabled} title={o.title}
          onClick={() => onChange(o.value)}
          className={cn("relative z-10 flex items-center justify-center gap-1.5 whitespace-nowrap rounded-[10px] font-semibold outline-none transition-colors duration-200 focus-visible:ring-2 focus-visible:ring-primary/40 disabled:opacity-40",
            size === "sm" ? "px-2.5 py-1 text-xs" : "px-3.5 py-1.5 text-[13px]", o.value === value ? "text-ink" : "text-slate-500 hover:text-ink")}>
          {o.label}
        </button>
      ))}
    </div>
  );
}

/** A labelled on/off switch with a thumb that slides; the whole row is the target. */
export function Switch({ checked, onChange, label, hint, className }: { checked: boolean; onChange: (v: boolean) => void; label: ReactNode; hint?: string; className?: string }) {
  const id = useId();
  return (
    <label htmlFor={id} title={hint} className={cn("flex cursor-pointer select-none items-center gap-2 text-xs font-medium text-slate-600", className)}>
      <input id={id} type="checkbox" role="switch" className="peer sr-only" checked={checked} onChange={(e) => onChange(e.target.checked)} />
      <span aria-hidden className="relative h-[18px] w-8 shrink-0 rounded-full bg-slate-200 transition-colors duration-300 peer-checked:bg-leaf peer-focus-visible:ring-2 peer-focus-visible:ring-primary/40
        after:absolute after:left-0.5 after:top-0.5 after:size-3.5 after:rounded-full after:bg-white after:shadow after:transition-transform after:duration-300 peer-checked:after:translate-x-3.5" />
      {label}
    </label>
  );
}

/** A check chip: a filter that is on or off, with a check mark that pops in. */
export function CheckChip({ checked, onChange, children, color }: { checked: boolean; onChange: (v: boolean) => void; children: ReactNode; color?: string }) {
  return (
    <button type="button" role="checkbox" aria-checked={checked} onClick={() => onChange(!checked)}
      className={cn("inline-flex items-center gap-1.5 rounded-full px-2.5 py-1 text-xs font-semibold ring-1 transition-all duration-200 focus-visible:outline-2",
        checked ? "bg-white text-ink ring-slate-300 shadow-sm" : "bg-slate-50 text-slate-400 ring-line hover:text-slate-600")}>
      <span aria-hidden className={cn("grid size-3.5 place-items-center rounded-[5px] ring-1 transition-all duration-200", checked ? "ring-transparent" : "bg-white ring-slate-300")} style={checked ? { background: color ?? "#5f8d6c" } : undefined}>
        {checked && <svg viewBox="0 0 12 12" className="pop size-2.5 text-white" fill="none" stroke="currentColor" strokeWidth="2.2" strokeLinecap="round" strokeLinejoin="round"><path d="m2.500 6.200 2.300 2.200 4.700-5" /></svg>}
      </span>
      {children}
    </button>
  );
}
