"use client";

import { useState } from "react";
import { cn } from "@/lib/utils";

/** Two-step button for destructive actions: the first click arms it, the second one performs it. */
export function ConfirmButton({ label, confirmLabel = "Confirm", onConfirm, disabled, className }:
  { label: string; confirmLabel?: string; onConfirm: () => void; disabled?: boolean; className?: string }) {
  const [armed, setArmed] = useState(false);
  return armed ? (
    <span className="inline-flex items-center gap-1.5">
      <button type="button" disabled={disabled} onClick={() => { setArmed(false); onConfirm(); }}
        className={cn("rounded-lg bg-bad px-2.5 py-1 text-xs font-semibold text-white disabled:opacity-60", className)}>{confirmLabel}</button>
      <button type="button" onClick={() => setArmed(false)} className="rounded-lg px-2 py-1 text-xs text-muted hover:bg-white">Cancel</button>
    </span>
  ) : (
    <button type="button" disabled={disabled} onClick={() => setArmed(true)}
      className={cn("rounded-lg px-2.5 py-1 text-xs font-medium text-bad ring-1 ring-red-200 hover:bg-red-50 disabled:opacity-60", className)}>{label}</button>
  );
}
