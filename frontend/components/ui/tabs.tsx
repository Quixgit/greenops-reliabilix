"use client";

import * as T from "@radix-ui/react-tabs";
import { cn } from "@/lib/utils";

export function PillTabs({ value, onChange, tabs, label = "Service category" }: { value: string; onChange: (v: string) => void; tabs: { value: string; label: string }[]; label?: string }) {
  return (
    <T.Root value={value} onValueChange={onChange}>
      <T.List className="flex gap-0.5 overflow-x-auto border-b border-[var(--dx-line-strong)] [scrollbar-width:none]" aria-label={label}>
        {tabs.map((t) => (
          <T.Trigger key={t.value} value={t.value}
            className="-mb-px h-10 shrink-0 border-b-2 border-transparent px-3.5 text-sm font-medium text-muted outline-none transition-colors hover:text-ink focus-visible:text-ink data-[state=active]:border-[var(--dx-navy)] data-[state=active]:font-semibold data-[state=active]:text-ink">
            {t.label}
          </T.Trigger>
        ))}
      </T.List>
    </T.Root>
  );
}
