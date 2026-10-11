"use client";

import { useEffect, useState, type ReactNode } from "react";
import { createPortal } from "react-dom";
import { CloseButton } from "@/components/ui/close-button";

/** A panel that slides in from the right over a dimmed page; Escape and a click outside close it. */
export function Drawer({ open, title, onClose, children, closeLabel }: { open: boolean; title: ReactNode; onClose: () => void; children: ReactNode; closeLabel: string }) {
  useEffect(() => {
    if (!open) return;
    const key = (e: KeyboardEvent) => { if (e.key === "Escape") onClose(); };
    document.addEventListener("keydown", key);
    const prev = document.body.style.overflow;
    document.body.style.overflow = "hidden";
    return () => { document.removeEventListener("keydown", key); document.body.style.overflow = prev; };
  }, [open, onClose]);
  // Rendered into the body: the page sits in a transformed, stacked container and the app header has its own layer,
  // so a fixed panel inside it would slide in under the header.
  const [mounted, setMounted] = useState(false);
  useEffect(() => setMounted(true), []);
  if (!open || !mounted) return null;
  return createPortal(
    <div className="fixed inset-0 z-[100]" role="dialog" aria-modal="true">
      <button aria-label={closeLabel} onClick={onClose} className="scrim-in absolute inset-0 bg-slate-900/35 backdrop-blur-[2px]" />
      <aside className="drawer-in absolute inset-y-0 right-0 flex w-full max-w-xl flex-col bg-white shadow-2xl ring-1 ring-line">
        <header className="flex items-center justify-between border-b border-line px-6 py-4">
          <h2 className="text-lg font-semibold tracking-tight">{title}</h2>
          <CloseButton onClick={onClose} label={closeLabel} />
        </header>
        <div className="flex-1 overflow-y-auto px-6 py-5">{children}</div>
      </aside>
    </div>,
    document.body,
  );
}
