import { cn } from "@/lib/utils";
import { IconClose } from "./icons";

/** The close control of every window and panel: a small soft circle with a hand-drawn cross that turns a quarter on hover. */
export function CloseButton({ onClick, label, className }: { onClick: () => void; label: string; className?: string }) {
  return (
    <button type="button" onClick={onClick} aria-label={label} title={label}
      className={cn("group grid size-7 shrink-0 place-items-center rounded-full bg-slate-100/80 text-slate-500 ring-1 ring-slate-200/70 transition-all duration-200 hover:bg-white hover:text-slate-900 hover:shadow-sm hover:ring-slate-300 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-leaf active:scale-95", className)}>
      <IconClose className="size-[14px] transition-transform duration-300 group-hover:rotate-90" strokeWidth={2} />
    </button>
  );
}
