"use client";

import * as T from "@radix-ui/react-tabs";
import { cn } from "@/lib/utils";

export function PillTabs({ value, onChange, tabs }: { value: string; onChange: (v: string) => void; tabs: { value: string; label: string }[] }) {
  return (
    <T.Root value={value} onValueChange={onChange}>
      <T.List className="flex flex-wrap gap-1" aria-label="Service category">
        {tabs.map((t) => (
          <T.Trigger key={t.value} value={t.value}
            className={cn("rounded-lg px-3 py-1.5 text-sm font-medium text-muted outline-none transition focus-visible:ring-2 focus-visible:ring-primary",
              "data-[state=active]:bg-primary/10 data-[state=active]:text-primary hover:text-ink")}>
            {t.label}
          </T.Trigger>
        ))}
      </T.List>
    </T.Root>
  );
}
