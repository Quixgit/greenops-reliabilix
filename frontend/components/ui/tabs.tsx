"use client";

import * as T from "@radix-ui/react-tabs";
import { cn } from "@/lib/utils";

export function PillTabs({ value, onChange, tabs, label = "Service category" }: { value: string; onChange: (v: string) => void; tabs: { value: string; label: string }[]; label?: string }) {
  return (
    <T.Root value={value} onValueChange={onChange}>
      <T.List className="flex flex-wrap gap-1" aria-label={label}>
        {tabs.map((t) => (
          <T.Trigger key={t.value} value={t.value}
            className={cn("rounded-lg px-3.5 py-1.5 text-sm font-medium text-muted outline-none ring-1 ring-transparent transition-all duration-200 focus-visible:ring-2 focus-visible:ring-leaf/50",
              "hover:text-ink data-[state=active]:bg-blue-50/70 data-[state=active]:text-primary data-[state=active]:ring-blue-200")}>
            {t.label}
          </T.Trigger>
        ))}
      </T.List>
    </T.Root>
  );
}
