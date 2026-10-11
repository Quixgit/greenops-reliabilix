"use client";

import * as DM from "@radix-ui/react-dropdown-menu";
import { useState } from "react";
import { cn } from "@/lib/utils";
import { Calendar } from "./calendar";
import { DropdownRoot } from "./dropdown";
import { IconClock } from "./icons";

/** A date input that opens the platform calendar; the value is an ISO date (YYYY-MM-DD). */
export function DateField({ value, onChange, min, max, className, label }: { value: string; onChange: (v: string) => void; min?: string; max?: string; className?: string; label?: string }) {
  const [open, setOpen] = useState(false);
  return (
    <DropdownRoot open={open} onOpenChange={setOpen}>
      <DM.Trigger aria-label={label} className={cn("flex w-full items-center justify-between gap-2 rounded-xl border border-line bg-white px-3 py-2 text-left text-sm tabular-nums outline-none transition-all hover:border-slate-300 data-[state=open]:border-leaf data-[state=open]:shadow-[0_0_0_3px_rgba(111,195,168,0.22)]", !value && "text-slate-400", className)}>
        <span>{value || "Pick a date"}</span><IconClock className="size-4 text-slate-400" />
      </DM.Trigger>
      <DM.Portal>
        <DM.Content align="start" sideOffset={8} className="z-50 rounded-2xl border border-line bg-white p-3 shadow-[0_12px_40px_rgba(31,52,110,0.16)] data-[state=open]:animate-[rise_0.22s_var(--ease-out-soft)]">
          <Calendar mode="single" from={value || undefined} min={min} max={max} onPick={(f) => { onChange(f); setOpen(false); }} />
        </DM.Content>
      </DM.Portal>
    </DropdownRoot>
  );
}
