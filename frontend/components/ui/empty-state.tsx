import type { LucideIcon } from "lucide-react";
import Link from "next/link";
import { cn } from "@/lib/utils";

/** The honest "no data" state: says why there is nothing and, where possible, what to do. */
export function EmptyState({
  icon: Icon, text, action, className,
}: { icon: LucideIcon; text: string; action?: { href: string; label: string }; className?: string }) {
  return (
    <div className={cn("flex flex-col items-center justify-center gap-2 py-8 text-center", className)}>
      <span className="flex size-10 items-center justify-center rounded-full bg-slate-100 text-slate-400">
        <Icon className="size-5" />
      </span>
      <p className="max-w-xs text-sm text-muted">{text}</p>
      {action && (
        <Link href={action.href} className="mt-1 rounded-lg bg-white px-3 py-1.5 text-sm font-medium text-primary shadow-sm ring-1 ring-line hover:bg-slate-50">
          {action.label}
        </Link>
      )}
    </div>
  );
}

export function ErrorState({ text = "Couldn't load this data. Try again in a moment." }: { text?: string }) {
  return <p role="alert" className="py-8 text-center text-sm text-bad">{text}</p>;
}
