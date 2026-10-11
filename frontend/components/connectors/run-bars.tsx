import type { CSSProperties } from "react";
import { cn } from "@/lib/utils";

const TONE: Record<string, string> = { succeeded: "bg-leaf", failed: "bg-pastel-rose", running: "bg-pastel-amber animate-pulse" };

/** The latest sync runs, oldest on the left: one bar each, green for a success, red for a failure. */
export function RunBars({ runs, limit = 12, className }: { runs: { status: string; started_at: string }[]; limit?: number; className?: string }) {
  const shown = runs.slice(0, limit).reverse();
  if (shown.length === 0) return <span className="text-xs text-muted">—</span>;
  return (
    <span className={cn("inline-flex h-6 items-end gap-0.5", className)} aria-hidden>
      {shown.map((r, i) => (
        <span key={`${r.started_at}-${i}`} title={`${r.status} · ${new Date(r.started_at).toLocaleString()}`}
          className={cn("bar-in block w-1.5 rounded-sm", TONE[r.status] ?? "bg-slate-300", r.status === "failed" ? "h-3" : "h-5")}
          style={{ "--i": i } as CSSProperties} />
      ))}
    </span>
  );
}
