"use client";

import * as DM from "@radix-ui/react-dropdown-menu";
import { IconChevronDown as ChevronDown } from "@/components/ui/icons";
import type { ReactNode } from "react";
import { cn } from "@/lib/utils";

/** Not modal: a modal menu locks the page scroll and pads the body, which made the layout jump when a menu opened. */
export function DropdownRoot(props: React.ComponentProps<typeof DM.Root>) {
  return <DM.Root modal={false} {...props} />;
}

export function DropdownTrigger({ children, className }: { children: ReactNode; className?: string }) {
  return (
    <DM.Trigger className={cn("group flex items-center gap-2 rounded-xl bg-white px-3.5 py-2 text-sm font-medium text-ink shadow-[0_1px_2px_rgba(16,24,40,0.04)] ring-1 ring-line outline-none transition hover:ring-slate-300 focus-visible:ring-2 focus-visible:ring-leaf/50 data-[state=open]:ring-leaf/50", className)}>
      {children}
      <ChevronDown className="size-4 text-muted transition-transform duration-300 group-data-[state=open]:rotate-180" />
    </DM.Trigger>
  );
}

export function DropdownPanel({ children, align = "start" }: { children: ReactNode; align?: "start" | "end" }) {
  return (
    <DM.Portal>
      <DM.Content align={align} sideOffset={8}
        className="z-50 min-w-52 origin-[var(--radix-dropdown-menu-content-transform-origin)] rounded-2xl border border-line bg-white p-1.5 text-sm shadow-[0_12px_40px_rgba(31,52,110,0.16)] data-[state=open]:animate-[rise_0.22s_var(--ease-out-soft)]">
        {children}
      </DM.Content>
    </DM.Portal>
  );
}

export const itemClass = "flex cursor-pointer items-center gap-2 rounded-lg px-3 py-2 outline-none transition-colors data-[highlighted]:bg-slate-100";
export const DropdownItem = ({ children, onSelect, asChild }: { children: ReactNode; onSelect?: () => void; asChild?: boolean }) => (
  <DM.Item onSelect={onSelect} asChild={asChild} className={itemClass}>{children}</DM.Item>
);
export const DropdownRadioGroup = DM.RadioGroup;
export const DropdownRadioItem = ({ value, children }: { value: string; children: ReactNode }) => (
  <DM.RadioItem value={value} className={cn(itemClass, "data-[state=checked]:font-semibold data-[state=checked]:text-primary")}>{children}</DM.RadioItem>
);
