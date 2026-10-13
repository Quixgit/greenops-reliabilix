"use client";

import { useEffect, useId, useMemo, useRef, useState, useSyncExternalStore, type CSSProperties, type MouseEvent as ReactMouseEvent, type ReactNode } from "react";
import { Ico, type DxIconName } from "./icons";

export const cx = (...a: (string | false | null | undefined)[]) => a.filter(Boolean).join(" ");

/** Closes on an outside press and on Escape; the trigger sets `aria-expanded`. */
export function usePopover() {
  const [open, setOpen] = useState(false);
  const root = useRef<HTMLSpanElement>(null);
  useEffect(() => {
    if (!open) return;
    const down = (e: MouseEvent) => { if (!root.current?.contains(e.target as Node)) setOpen(false); };
    const key = (e: KeyboardEvent) => { if (e.key === "Escape") setOpen(false); };
    document.addEventListener("mousedown", down);
    document.addEventListener("keydown", key);
    return () => { document.removeEventListener("mousedown", down); document.removeEventListener("keydown", key); };
  }, [open]);
  return { open, setOpen, root };
}

/** A pop-over opened by a trigger: `trigger(open, toggle)` draws the button, `children` is what opens. */
export function Popover({ trigger, children, align = "left", up = false, className }: {
  trigger: (open: boolean, toggle: (e?: ReactMouseEvent<HTMLElement>) => void) => ReactNode; children: ReactNode | ((close: () => void) => ReactNode); align?: "left" | "right" | "auto"; up?: boolean; className?: string;
}) {
  const { open, setOpen, root } = usePopover();
  const [side, setSide] = useState<"left" | "right">("left");
  const toggle = (e?: ReactMouseEvent<HTMLElement>) => {
    // "auto": open towards the side that has room, measured from the button that was pressed
    if (!open && align === "auto" && e) {
      const r = e.currentTarget.getBoundingClientRect();
      setSide(window.innerWidth - r.left < 400 ? "right" : "left");
    }
    setOpen((o) => !o);
  };
  const place = align === "auto" ? side : align;
  return (
    <span className="dx-pop-root" ref={root}>
      {trigger(open, toggle)}
      {open && <div className={cx("dx-pop", place === "right" ? "is-right" : "is-left", up && "is-up", className)} role="dialog">{typeof children === "function" ? children(() => setOpen(false)) : children}</div>}
    </span>
  );
}

/** The small "i": what a figure means, in one short card. */
export function Hint({ children, title }: { children: ReactNode; title?: string }) {
  return (
    <Popover align="auto" trigger={(open, toggle) => (
      <button type="button" className="dx-info" aria-expanded={open} aria-label={title ?? "Info"} onClick={toggle}><Ico name="info" size={15} /></button>
    )}>
      <div className="dx-pop-body">{title && <p style={{ fontWeight: 600, color: "var(--dx-ink)" }}>{title}</p>}<p>{children}</p></div>
    </Popover>
  );
}

export function Segmented<V extends string>({ value, onChange, options, small, label }: { value: V; onChange: (v: V) => void; options: { value: V; label: ReactNode }[]; small?: boolean; label?: string }) {
  const i = Math.max(0, options.findIndex((o) => o.value === value));
  return (
    <div className={cx("dx-seg", small && "is-sm")} role="group" aria-label={label} style={{ "--n": options.length, "--i": i } as CSSProperties}>
      <span className="dx-seg-ind" aria-hidden />
      {options.map((o) => <button key={o.value} type="button" aria-pressed={o.value === value} onClick={() => onChange(o.value)}>{o.label}</button>)}
    </div>
  );
}

export function Switch({ checked, onChange, label, title }: { checked: boolean; onChange: (v: boolean) => void; label: ReactNode; title?: string }) {
  return (
    <label className="dx-switch" title={title}>
      <input type="checkbox" role="switch" checked={checked} onChange={(e) => onChange(e.target.checked)} />
      <span className="dx-track" aria-hidden />
      <span>{label}</span>
    </label>
  );
}

export function Check({ checked, onChange, label, color, disabled }: { checked: boolean; onChange: (v: boolean) => void; label: ReactNode; color?: string; disabled?: boolean }) {
  return (
    <label className="dx-check" style={disabled ? { opacity: 0.55, cursor: "default" } : undefined}>
      <input type="checkbox" checked={checked} disabled={disabled} onChange={(e) => onChange(e.target.checked)} />
      <span className="dx-box" aria-hidden><svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="3.2" strokeLinecap="square"><path d="M4 12.5l5 5L20 6.5" /></svg></span>
      {color && <span className="dx-swatch" style={{ background: color }} aria-hidden />}
      <span>{label}</span>
    </label>
  );
}

/** A single choice from a list, in a pop-over. */
export function Select<V extends string>({ value, onChange, options, icon, label, align }: { value: V; onChange: (v: V) => void; options: { value: V; label: string }[]; icon?: DxIconName; label: string; align?: "left" | "right" }) {
  const current = options.find((o) => o.value === value);
  return (
    <Popover align={align} trigger={(open, toggle) => (
      <button type="button" className="dx-field" aria-expanded={open} aria-haspopup="listbox" aria-label={label} onClick={toggle}>
        {icon && <Ico name={icon} size={18} />}
        <span style={{ maxWidth: 180, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>{current?.label ?? label}</span>
        <Ico name="chevronDown" size={14} className="dx-chev" />
      </button>
    )}>
      {(close) => (
        <div role="listbox" aria-label={label} style={{ minWidth: 200, padding: "4px 0" }}>
          {options.map((o) => (
            <button key={o.value} type="button" role="option" aria-selected={o.value === value} className="dx-opt" onClick={() => { onChange(o.value); close(); }}>
              <span>{o.label}</span>{o.value === value && <Ico name="check" size={16} className="dx-chk" />}
            </button>
          ))}
        </div>
      )}
    </Popover>
  );
}

export function Panel({ icon, tone, title, sub, tools, toolbar, hint, children, index = 0, flush }: {
  icon?: DxIconName; tone?: "sage" | "sky" | "rose" | "butter" | "peach" | "navy"; title: ReactNode; sub?: ReactNode; tools?: ReactNode; toolbar?: ReactNode; hint?: ReactNode; children: ReactNode; index?: number; flush?: boolean;
}) {
  return (
    <section className="dx-panel dx-rise" style={{ "--i": index } as CSSProperties}>
      <header className="dx-ph">
        {icon && <span className={cx("dx-ico", tone && `is-${tone}`)}><Ico name={icon} size={18} /></span>}
        <h2>{title}</h2>
        {sub && <span className="dx-sub">{sub}</span>}
        <div className="dx-ph-tools">{tools}{hint && <Hint>{hint}</Hint>}</div>
      </header>
      {toolbar && <div className="dx-toolbar">{toolbar}</div>}
      <div className={flush ? undefined : "dx-pb"}>{children}</div>
    </section>
  );
}

export function Delta({ pct, goodWhenDown = true }: { pct: number | null; goodWhenDown?: boolean }) {
  if (pct === null || !Number.isFinite(pct)) return null;
  const flat = Math.abs(pct) < 0.5;
  const good = flat ? null : goodWhenDown ? pct < 0 : pct > 0;
  return (
    <span className={cx("dx-delta", flat ? "is-flat" : good ? "is-good" : "is-bad")}>
      <Ico name={pct >= 0 ? "arrowUp" : "arrowDown"} size={13} strokeWidth={2.2} />{Math.abs(pct).toFixed(Math.abs(pct) < 10 ? 1 : 0)}%
    </span>
  );
}

/** A flat sparkline: one polyline, drawn on load. */
export function Spark({ values, color = "var(--dx-cost)", width = 84, height = 30 }: { values: number[]; color?: string; width?: number; height?: number }) {
  if (values.length < 2) return null;
  const min = Math.min(...values), max = Math.max(...values), span = max - min || 1;
  const pts = values.map((v, i) => `${((i / (values.length - 1)) * (width - 2) + 1).toFixed(1)},${(height - 3 - ((v - min) / span) * (height - 6)).toFixed(1)}`).join(" ");
  return (
    <svg viewBox={`0 0 ${width} ${height}`} width={width} height={height} aria-hidden style={{ overflow: "visible" }}>
      <polyline points={pts} pathLength={1} fill="none" stroke={color} strokeWidth="1.6" strokeLinejoin="miter" style={{ strokeDasharray: 1, strokeDashoffset: 1, animation: "dx-draw 0.9s var(--dx-ease) 0.15s forwards" }} />
    </svg>
  );
}

export function Bar({ pct, color, index = 0, thin }: { pct: number; color?: string; index?: number; thin?: boolean }) {
  return <div className={cx("dx-bar", thin && "is-thin")} style={{ "--c": color, "--i": index } as CSSProperties}><i style={{ width: `${Math.max(0, Math.min(100, pct))}%` }} /></div>;
}

export function Skel({ h = 16, w = "100%" }: { h?: number; w?: number | string }) {
  return <div className="dx-skel" style={{ height: h, width: w }} aria-hidden />;
}

export function Tag({ tone, children, title }: { tone?: "sage" | "sky" | "rose" | "butter" | "peach" | "line"; children: ReactNode; title?: string }) {
  return <span className={cx("dx-tag", tone && `is-${tone}`)} title={title}>{children}</span>;
}

export function Empty({ icon, children }: { icon: DxIconName; children: ReactNode }) {
  return <div className="dx-empty"><div style={{ display: "grid", placeItems: "center", marginBottom: 8, color: "var(--dx-line-strong)" }}><Ico name={icon} size={28} /></div>{children}</div>;
}

const listeners = new Set<() => void>();
const subscribe = (cb: () => void) => { listeners.add(cb); window.addEventListener("storage", cb); return () => { listeners.delete(cb); window.removeEventListener("storage", cb); }; };

/** Remembers a choice on this device (read without a flash: the server renders the initial value, the browser then reads its own). */
export function useStored<T>(key: string, initial: T): [T, (v: T) => void] {
  const raw = useSyncExternalStore(
    subscribe,
    () => { try { return localStorage.getItem(`dx:${key}`); } catch { return null; } },
    () => null,
  );
  const value = useMemo<T>(() => { try { return raw === null ? initial : (JSON.parse(raw) as T); } catch { return initial; } }, [raw, initial]);
  const set = (next: T) => {
    try { localStorage.setItem(`dx:${key}`, JSON.stringify(next)); } catch { /* private mode */ }
    listeners.forEach((l) => l());
  };
  return [value, set];
}

export function useUid() { return useId(); }
