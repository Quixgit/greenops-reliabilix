"use client";

import { useState } from "react";
import { cn } from "@/lib/utils";

const TONE = {
  danger: { idle: "text-[#a8504b] ring-rose-200 hover:bg-rose-50", armed: "bg-[#b8625c] text-white hover:bg-[#cf6272]" },
  primary: { idle: "text-leaf-deep ring-leaf/40 hover:bg-leaf-soft/60", armed: "bg-leaf-deep text-white hover:bg-[#277a66]" },
} as const;

/**
 * Two-step button: the first click arms it, the second one performs it. `danger` (default) is for what cannot be taken
 * back, `primary` for a deliberate action that reaches other people, such as sending an announcement.
 */
export function ConfirmButton({ label, confirmLabel = "Confirm", onConfirm, disabled, className, tone = "danger" }:
  { label: string; confirmLabel?: string; onConfirm: () => void; disabled?: boolean; className?: string; tone?: keyof typeof TONE }) {
  const [armed, setArmed] = useState(false);
  return armed ? (
    <span className="rise inline-flex items-center gap-1.5">
      <button type="button" disabled={disabled} onClick={() => { setArmed(false); onConfirm(); }}
        className={cn("rounded-xl px-3.5 py-2 text-xs font-semibold transition disabled:opacity-60", TONE[tone].armed, className)}>{confirmLabel}</button>
      <button type="button" onClick={() => setArmed(false)} className="rounded-xl px-3 py-2 text-xs font-medium text-muted transition hover:bg-slate-100">Cancel</button>
    </span>
  ) : (
    <button type="button" disabled={disabled} onClick={() => setArmed(true)}
      className={cn("rounded-xl bg-white px-3.5 py-2 text-xs font-semibold ring-1 transition disabled:cursor-not-allowed disabled:opacity-50", TONE[tone].idle, className)}>{label}</button>
  );
}
