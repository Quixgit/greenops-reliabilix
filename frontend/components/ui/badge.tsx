import type { ReactNode } from "react";
import { cn } from "@/lib/utils";

type Tone = "good" | "warn" | "bad" | "neutral" | "stub";
const TONES: Record<Tone, string> = {
  good: "bg-leaf-soft text-leaf-deep ring-leaf-soft",
  warn: "bg-amber-50 text-[#77621a] ring-amber-200",
  bad: "bg-rose-50 text-[#a8504b] ring-rose-200",
  neutral: "bg-slate-100 text-slate-600 ring-slate-200",
  stub: "bg-violet-50 text-violet-700 ring-violet-200",
};

export function Badge({ tone = "neutral", children, className }: { tone?: Tone; children: ReactNode; className?: string }) {
  return <span className={cn("inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium ring-1 ring-inset", TONES[tone], className)}>{children}</span>;
}

export const statusTone = (s: string): Tone =>
  ({ sent: "good", ready: "good", healthy: "good", pending: "warn", failed: "bad", error: "bad", missing: "bad", skipped: "neutral", stub: "stub" } as Record<string, Tone>)[s] ?? "neutral";
