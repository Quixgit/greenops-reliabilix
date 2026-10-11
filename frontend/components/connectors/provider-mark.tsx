import { cn } from "@/lib/utils";

const MARKS: Record<string, { label: string; tile: string }> = {
  aws: { label: "AWS", tile: "from-amber-400 to-orange-500" },
  azure: { label: "Az", tile: "from-sky-400 to-blue-600" },
  gcp: { label: "GC", tile: "from-emerald-400 to-teal-600" },
  kubernetes: { label: "K8s", tile: "from-indigo-400 to-blue-600" },
  email: { label: "@", tile: "from-slate-400 to-slate-600" },
  slack: { label: "#", tile: "from-fuchsia-400 to-violet-600" },
  webhook: { label: "{ }", tile: "from-cyan-400 to-sky-600" },
  teams: { label: "T", tile: "from-indigo-400 to-violet-600" },
};

/** One tile per connector: the same shape and weight for every provider, so a list of mixed sources reads evenly. */
export function ProviderMark({ id, size = "md", dim = false }: { id: string; size?: "sm" | "md" | "lg"; dim?: boolean }) {
  const m = MARKS[id] ?? { label: id.slice(0, 2).toUpperCase(), tile: "from-slate-300 to-slate-500" };
  return (
    <span
      aria-hidden
      className={cn(
        "inline-flex shrink-0 items-center justify-center rounded-xl bg-gradient-to-br font-bold tracking-tight text-white shadow-sm ring-1 ring-black/5 transition-transform duration-300",
        size === "sm" ? "size-8 text-[11px]" : size === "lg" ? "size-12 text-sm" : "size-10 text-xs",
        m.tile, dim && "opacity-45 saturate-50",
      )}
    >
      {m.label}
    </span>
  );
}
