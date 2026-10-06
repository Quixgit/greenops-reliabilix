"use client";

import * as DM from "@radix-ui/react-dropdown-menu";
import { ChevronDown } from "lucide-react";
import type { ReactNode } from "react";
import { cn } from "@/lib/utils";

export const DropdownRoot = DM.Root;

export function DropdownTrigger({ children, className }: { children: ReactNode; className?: string }) {
  return (
    <DM.Trigger className={cn("flex items-center gap-2 rounded-xl bg-white/80 px-3.5 py-2 text-sm font-medium shadow-sm ring-1 ring-line outline-none hover:bg-white focus-visible:ring-2 focus-visible:ring-primary", className)}>
      {children}
      <ChevronDown className="size-4 text-muted" />
    </DM.Trigger>
  );
}

export function DropdownPanel({ children, align = "start" }: { children: ReactNode; align?: "start" | "end" }) {
  return (
    <DM.Portal>
      <DM.Content align={align} sideOffset={8} className="glass z-50 min-w-48 p-1.5 text-sm">
        {children}
      </DM.Content>
    </DM.Portal>
  );
}

export const itemClass = "flex cursor-pointer items-center gap-2 rounded-lg px-3 py-2 outline-none data-[highlighted]:bg-slate-100";
export const DropdownItem = ({ children, onSelect, asChild }: { children: ReactNode; onSelect?: () => void; asChild?: boolean }) => (
  <DM.Item onSelect={onSelect} asChild={asChild} className={itemClass}>{children}</DM.Item>
);
export const DropdownRadioGroup = DM.RadioGroup;
export const DropdownRadioItem = ({ value, children }: { value: string; children: ReactNode }) => (
  <DM.RadioItem value={value} className={cn(itemClass, "data-[state=checked]:font-semibold data-[state=checked]:text-primary")}>{children}</DM.RadioItem>
);
