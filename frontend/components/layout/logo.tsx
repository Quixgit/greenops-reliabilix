import { cn } from "@/lib/utils";

/** Two intersecting skewed shapes, blue -> violet gradient. */
export function Logo({ className }: { className?: string }) {
  return (
    <svg viewBox="0 0 40 40" className={cn("size-9", className)} role="img" aria-label="Reliabilix">
      <defs>
        <linearGradient id="lg" x1="0" y1="0" x2="1" y2="1"><stop offset="0" stopColor="#3b82f6" /><stop offset="1" stopColor="#7c4dff" /></linearGradient>
      </defs>
      <path d="M6 8h14l-8 24H0z" fill="url(#lg)" opacity=".95" transform="translate(2 0)" />
      <path d="M20 8h14l-8 24H12z" fill="url(#lg)" opacity=".6" transform="translate(4 0)" />
    </svg>
  );
}
