"use client";

import { useMemo, useState } from "react";
import { cn } from "@/lib/utils";
import { IconChevronRight } from "./icons";

const DOW = ["Mo", "Tu", "We", "Th", "Fr", "Sa", "Su"];
const iso = (d: Date) => `${d.getUTCFullYear()}-${String(d.getUTCMonth() + 1).padStart(2, "0")}-${String(d.getUTCDate()).padStart(2, "0")}`;
const parse = (s: string) => { const [y, m, d] = s.split("-").map(Number); return new Date(Date.UTC(y, m - 1, d)); };
const today = () => { const n = new Date(); return iso(new Date(Date.UTC(n.getFullYear(), n.getMonth(), n.getDate()))); };

export interface CalendarProps {
  /** Single date or the two ends of a range (a range with only `from` is a pick in progress). */
  mode: "single" | "range";
  from?: string; to?: string;
  onPick: (from: string, to?: string) => void;
  min?: string; max?: string;
}

/** A month grid in the platform's own look: soft leaf for the chosen days, a calm band for a range, a month that slides. */
export function Calendar({ mode, from, to, onPick, min, max }: CalendarProps) {
  const [view, setView] = useState(() => { const d = from ? parse(from) : parse(today()); return new Date(Date.UTC(d.getUTCFullYear(), d.getUTCMonth(), 1)); });
  const [dir, setDir] = useState(0);
  const [hover, setHover] = useState<string | null>(null);
  const days = useMemo(() => {
    const first = view.getUTCDay() === 0 ? 6 : view.getUTCDay() - 1; // Monday first
    const dim = new Date(Date.UTC(view.getUTCFullYear(), view.getUTCMonth() + 1, 0)).getUTCDate();
    return [...Array(first).fill(null), ...Array.from({ length: dim }, (_, i) => iso(new Date(Date.UTC(view.getUTCFullYear(), view.getUTCMonth(), i + 1))))] as (string | null)[];
  }, [view]);
  const move = (n: number) => { setDir(n); setView(new Date(Date.UTC(view.getUTCFullYear(), view.getUTCMonth() + n, 1))); };
  const t0 = today();
  const end = mode === "range" ? (to ?? (from && hover && hover >= from ? hover : undefined)) : undefined;
  const pick = (d: string) => {
    if (mode === "single") return onPick(d);
    if (!from || to) return onPick(d); // start a new range
    if (d < from) return onPick(d, from);
    onPick(from, d);
  };
  const ok = (d: string) => (!min || d >= min) && (!max || d <= max);
  const title = view.toLocaleDateString(undefined, { month: "long", year: "numeric", timeZone: "UTC" });
  const inRange = (d: string) => !!from && !!end && d >= from && d <= end;

  return (
    <div className="w-[284px] select-none p-1">
      <div className="mb-2 flex items-center justify-between">
        <button type="button" aria-label="Previous month" onClick={() => move(-1)} className="grid size-8 place-items-center rounded-lg text-slate-500 transition hover:bg-slate-100 hover:text-ink"><IconChevronRight className="size-4 rotate-180" /></button>
        <p key={title} className="cal-title text-sm font-semibold capitalize text-ink">{title}</p>
        <button type="button" aria-label="Next month" onClick={() => move(1)} className="grid size-8 place-items-center rounded-lg text-slate-500 transition hover:bg-slate-100 hover:text-ink"><IconChevronRight className="size-4" /></button>
      </div>
      <div className="grid grid-cols-7 text-center text-[11px] font-medium text-slate-400">{DOW.map((d) => <span key={d} className="py-1">{d}</span>)}</div>
      <div key={title} className={cn("grid grid-cols-7 gap-y-0.5", dir > 0 ? "cal-next" : dir < 0 ? "cal-prev" : "")} onMouseLeave={() => setHover(null)}>
        {days.map((d, i) => {
          if (!d) return <span key={`e${i}`} />;
          const edge = d === from || d === (to ?? undefined) || (mode === "range" && !to && d === hover && !!from && d >= from);
          const mid = inRange(d) && !edge;
          const disabled = !ok(d);
          return (
            <button key={d} type="button" disabled={disabled} onClick={() => pick(d)} onMouseEnter={() => setHover(d)}
              className={cn("relative h-9 text-[13px] tabular-nums transition-all duration-150 disabled:cursor-not-allowed disabled:text-slate-300",
                mid && "bg-leaf-soft text-leaf-deep", mode === "range" && inRange(d) && d === from && "rounded-l-full", mode === "range" && inRange(d) && d === end && "rounded-r-full",
                edge ? "z-10 rounded-full bg-leaf-deep font-semibold text-white shadow-sm" : !mid && "rounded-full text-slate-700 hover:bg-slate-100",
                d === t0 && !edge && "font-semibold text-leaf-deep ring-1 ring-leaf/60")}>
              {Number(d.slice(8))}
            </button>
          );
        })}
      </div>
    </div>
  );
}
