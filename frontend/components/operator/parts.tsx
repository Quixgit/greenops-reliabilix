import type { AnyIcon as LucideIcon } from "@/components/ui/icons";
import type { ReactNode } from "react";
import { CountUp } from "@/components/ui/count-up";
import { cn } from "@/lib/utils";

export const th = "py-1.5 pr-3 text-left text-xs font-medium text-muted";
export const selectCls = "rounded-lg bg-white/80 px-2.5 py-1.5 text-sm ring-1 ring-line outline-none focus:ring-2 focus:ring-primary";

const TILE = {
  sage: "bg-emerald-50 text-emerald-700", lavender: "bg-violet-50 text-violet-700", peach: "bg-orange-50 text-orange-700",
  sky: "bg-sky-50 text-sky-700", butter: "bg-amber-50 text-amber-700",
} as const;
export type TileColor = keyof typeof TILE;

export function PageTitle({ title, sub, action }: { title: string; sub?: ReactNode; action?: ReactNode }) {
  return (
    <div className="flex flex-wrap items-end justify-between gap-3">
      <div>
        <h1 className="text-2xl font-bold tracking-tight">{title}</h1>
        {sub && <p className="mt-1 max-w-3xl text-sm text-muted">{sub}</p>}
      </div>
      {action}
    </div>
  );
}

/** A numeric tile. `value` null means "not known yet" and shows a dash, never a zero. `tone` marks it when it needs a look. */
export function Tile({ label, value, icon: Icon, color = "sage", tone, note, index }: {
  label: string; value: number | null | undefined; icon?: LucideIcon; color?: TileColor; tone?: "bad" | "warn"; note?: string; index?: number;
}) {
  return (
    <div className="glass lift rise p-4" style={{ "--i": index ?? 0 } as React.CSSProperties}>
      <div className="flex items-start justify-between gap-2">
        <p className="text-xs font-medium text-muted">{label}</p>
        {Icon && <span className={cn("flex size-8 items-center justify-center rounded-xl ", TILE[color])}><Icon className="size-4" aria-hidden /></span>}
      </div>
      <p className={cn("mt-2 text-3xl font-bold tracking-tight", tone === "bad" && "text-bad", tone === "warn" && "text-amber-700")}>
        {value == null ? "—" : <CountUp value={value} format={(n) => Math.round(n).toLocaleString()} />}
      </p>
      {note && <p className="mt-1 text-xs text-muted">{note}</p>}
    </div>
  );
}

export function Chip({ children, className }: { children: ReactNode; className?: string }) {
  return <span className={cn("inline-flex items-center rounded-full bg-white/80 px-2.5 py-0.5 text-xs text-slate-600 ring-1 ring-line", className)}>{children}</span>;
}
