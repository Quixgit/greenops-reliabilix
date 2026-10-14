"use client";

import { useEffect, useId, useRef, useState, type ReactNode } from "react";
import { createPortal } from "react-dom";
import { cn } from "@/lib/utils";
import { IconInfo } from "./icons";

const W = 304;

/**
 * The ⓘ of a block: what the figure is and how it works, in a small white card that opens on hover, focus or tap. With `corner` the icon
 * sits in the top-right corner of the block (the nearest `info-host`), the same place on every block of every page. The card is drawn at
 * the end of the page so no block with hidden overflow can cut it, keeps inside the window, and points an arrow at the icon.
 */
export function Info({ title, children, className, label = "More information", corner = false }: { title?: string; children: ReactNode; className?: string; label?: string; corner?: boolean }) {
  const id = useId();
  const btn = useRef<HTMLButtonElement>(null);
  const [pos, setPos] = useState<{ x: number; y: number; above: boolean; arrow: number } | null>(null);

  const show = () => {
    const r = btn.current?.getBoundingClientRect();
    if (!r) return;
    const x = Math.max(8, Math.min(window.innerWidth - W - 8, r.left + r.width / 2 - W / 2));
    const above = r.bottom + 200 > window.innerHeight && r.top > 200;
    const arrow = Math.max(14, Math.min(W - 14, r.left + r.width / 2 - x));
    setPos({ x, y: above ? r.top - 10 : r.bottom + 10, above, arrow });
  };
  const hide = () => setPos(null);
  useEffect(() => {
    if (!pos) return;
    const key = (e: KeyboardEvent) => { if (e.key === "Escape") hide(); };
    window.addEventListener("keydown", key);
    window.addEventListener("scroll", hide, true);
    return () => { window.removeEventListener("keydown", key); window.removeEventListener("scroll", hide, true); };
  }, [pos]);

  const button = (
    <button ref={btn} type="button" aria-label={label} aria-describedby={pos ? id : undefined}
      onMouseEnter={show} onMouseLeave={hide} onFocus={show} onBlur={hide} onClick={(e) => { e.preventDefault(); e.stopPropagation(); if (pos) hide(); else show(); }}
      className={cn("inline-grid size-[22px] shrink-0 cursor-help place-items-center rounded-full align-middle text-slate-400 transition-all duration-200 hover:bg-slate-100 hover:text-primary focus-visible:bg-slate-100 focus-visible:text-primary", pos && "bg-slate-100 text-primary", className)}>
      <svg viewBox="0 0 24 24" className="size-[15px]" fill="none" stroke="currentColor" strokeWidth="1.7" strokeLinecap="round" aria-hidden><circle cx="12" cy="12" r="9" /><path d="M12 11v5.5" /><circle cx="12" cy="7.6" r=".6" fill="currentColor" /></svg>
    </button>
  );
  return (
    <>
      {corner ? <span className="info-corner absolute right-2 top-2 z-10">{button}</span> : button}
      {pos && createPortal(
        <div id={id} role="tooltip" style={{ left: pos.x, top: pos.y, width: W, transform: pos.above ? "translateY(-100%)" : undefined }}
          className="info-pop pointer-events-none fixed z-[200] rounded-2xl bg-white p-3.5 text-left text-[12.5px] font-normal leading-relaxed text-slate-600 shadow-[0_18px_48px_rgba(31,52,110,.20),0_2px_8px_rgba(31,52,110,.08)] ring-1 ring-slate-200/80">
          <span aria-hidden className={cn("absolute size-3 rotate-45 bg-white ring-1 ring-slate-200/80", pos.above ? "-bottom-1.5" : "-top-1.5")} style={{ left: pos.arrow - 6, clipPath: pos.above ? "polygon(100% 0, 100% 100%, 0 100%)" : "polygon(0 0, 100% 0, 0 100%)" }} />
          <span className="relative block">
            <span className="mb-1.5 flex items-start gap-2.5">
              <span className="mt-0.5 grid size-6 shrink-0 place-items-center rounded-lg bg-primary/10 text-primary"><IconInfo className="size-[14px]" strokeWidth={1.8} /></span>
              <span className="min-w-0">{title && <span className="mb-0.5 block text-[13px] font-semibold text-ink">{title}</span>}{children}</span>
            </span>
          </span>
        </div>,
        document.body,
      )}
    </>
  );
}
