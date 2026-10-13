import type { AnyIcon as LucideIcon } from "@/components/ui/icons";
import type { ReactNode } from "react";
import { CountUp } from "@/components/ui/count-up";
import { cn } from "@/lib/utils";

export const th = "py-1.5 pr-3 text-left text-xs font-medium text-muted";
export const selectCls = "rounded-lg bg-white/80 px-2.5 py-1.5 text-sm ring-1 ring-line outline-none focus:ring-2 focus:ring-leaf/40";

// The drawn icon of a tile, in the same tones the client's cards use.
const TILE = {
  sage: "text-leaf-deep", lavender: "text-[#5a6aa0]", peach: "text-[#b8625c]", sky: "text-[#5d7fb0]", butter: "text-[#cba649]",
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

/**
 * A numeric tile, drawn like the client's cards: a large icon, the name and the number. `value` null means "not known yet"
 * and shows a dash, never a zero. `tone` marks it when it needs a look.
 */
export function Tile({ label, value, icon: Icon, color = "sage", tone, note, index }: {
  label: string; value: number | null | undefined; icon?: LucideIcon; color?: TileColor; tone?: "bad" | "warn"; note?: string; index?: number;
}) {
  return (
    <div className="glass lift rise group p-4" style={{ "--i": index ?? 0 } as React.CSSProperties}>
      <div className="flex items-center gap-3.5">
        {Icon && <Icon className={cn("size-10 shrink-0 transition-transform duration-500 ease-[var(--ease-out-soft)] group-hover:scale-110 group-hover:rotate-3", TILE[color])} strokeWidth={1.4} aria-hidden />}
        <div className="min-w-0">
          <p className="text-sm font-medium text-slate-700">{label}</p>
          <p className={cn("text-2xl font-bold leading-tight tracking-tight", tone === "bad" && "text-[#b8625c]", tone === "warn" && "text-[#cf9a3c]")}>
            {value == null ? <span className="text-slate-300">—</span> : <CountUp value={value} format={(n) => Math.round(n).toLocaleString()} />}
          </p>
        </div>
      </div>
      {note && <p className="mt-2.5 text-xs text-muted">{note}</p>}
    </div>
  );
}

/** The same tile as `Tile` for a value that is text (a duration, a time, "0.3 ms"). */
export function TextTile({ label, text, icon: Icon, color = "sage", note, index }: { label: string; text: string; icon?: LucideIcon; color?: TileColor; note?: string; index?: number }) {
  return (
    <div className="glass lift rise group p-4" style={{ "--i": index ?? 0 } as React.CSSProperties}>
      <div className="flex items-center gap-3.5">
        {Icon && <Icon className={cn("size-10 shrink-0 transition-transform duration-500 ease-[var(--ease-out-soft)] group-hover:scale-110 group-hover:rotate-3", TILE[color])} strokeWidth={1.4} aria-hidden />}
        <div className="min-w-0"><p className="text-sm font-medium text-slate-700">{label}</p><p className="text-2xl font-bold leading-tight tracking-tight">{text}</p></div>
      </div>
      {note && <p className="mt-2.5 text-xs text-muted">{note}</p>}
    </div>
  );
}

export function Chip({ children, className }: { children: ReactNode; className?: string }) {
  return <span className={cn("inline-flex items-center rounded-full bg-white/80 px-2.5 py-0.5 text-xs text-slate-600 ring-1 ring-line", className)}>{children}</span>;
}
