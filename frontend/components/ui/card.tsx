import type { CSSProperties, HTMLAttributes, ReactNode } from "react";
import { cn } from "@/lib/utils";

/**
 * White card with a soft shadow. `index` staggers the entrance animation (cards rise one after another);
 * `interactive` adds the hover lift for cards that react to the pointer.
 */
export function Card({ className, index, interactive = true, style, ...p }: HTMLAttributes<HTMLDivElement> & { index?: number; interactive?: boolean }) {
  return (
    <section
      className={cn("glass p-5", index !== undefined && "rise", interactive && "lift", className)}
      style={index !== undefined ? ({ "--i": index, ...style } as CSSProperties) : style}
      {...p}
    />
  );
}

export function CardHeader({ title, action, icon }: { title: ReactNode; action?: ReactNode; icon?: ReactNode }) {
  return (
    <div className="mb-4 flex items-center justify-between gap-3">
      <h2 className="flex items-center gap-2 text-[15px] font-semibold tracking-tight text-ink">{icon}{title}</h2>
      {action}
    </div>
  );
}
