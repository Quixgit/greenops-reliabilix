"use client";

import { useEffect, useId, useRef, useState, type ReactNode } from "react";
import { createPortal } from "react-dom";
import { cn } from "@/lib/utils";
import { IconInfo } from "./icons";

const W = 288;

/**
 * The ⓘ next to a number or a heading: what it is and how it works, on hover or focus (and on tap). The bubble is
 * drawn at the end of the page so no card with hidden overflow can cut it, and it keeps inside the window.
 */
export function Info({ title, children, className, label = "More information" }: { title?: string; children: ReactNode; className?: string; label?: string }) {
  const id = useId();
  const btn = useRef<HTMLButtonElement>(null);
  const [pos, setPos] = useState<{ x: number; y: number; above: boolean } | null>(null);

  const show = () => {
    const r = btn.current?.getBoundingClientRect();
    if (!r) return;
    const x = Math.max(8, Math.min(window.innerWidth - W - 8, r.left + r.width / 2 - W / 2));
    const above = r.bottom + 190 > window.innerHeight && r.top > 190;
    setPos({ x, y: above ? r.top - 8 : r.bottom + 8, above });
  };
  const hide = () => setPos(null);
  useEffect(() => {
    if (!pos) return;
    const key = (e: KeyboardEvent) => { if (e.key === "Escape") hide(); };
    window.addEventListener("keydown", key);
    window.addEventListener("scroll", hide, true);
    return () => { window.removeEventListener("keydown", key); window.removeEventListener("scroll", hide, true); };
  }, [pos]);

  return (
    <>
      <button ref={btn} type="button" aria-label={label} aria-describedby={pos ? id : undefined}
        onMouseEnter={show} onMouseLeave={hide} onFocus={show} onBlur={hide} onClick={(e) => { e.preventDefault(); e.stopPropagation(); pos ? hide() : show(); }}
        className={cn("inline-grid size-[18px] shrink-0 cursor-help place-items-center rounded-full align-middle text-slate-400 transition-colors hover:text-teal-700 focus-visible:text-teal-700", className)}>
        <IconInfo className="size-[15px]" strokeWidth={1.7} />
      </button>
      {pos && createPortal(
        <div id={id} role="tooltip" style={{ left: pos.x, top: pos.y, width: W, transform: pos.above ? "translateY(-100%)" : undefined }}
          className="info-pop pointer-events-none fixed z-[200] rounded-xl bg-slate-900/95 px-3.5 py-3 text-left text-xs font-normal leading-relaxed text-slate-100 shadow-2xl ring-1 ring-white/10 backdrop-blur">
          {title && <p className="mb-1 text-[13px] font-semibold text-white">{title}</p>}
          {children}
        </div>,
        document.body,
      )}
    </>
  );
}
