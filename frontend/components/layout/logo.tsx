import { cn } from "@/lib/utils";

/** Two intersecting skewed shapes, blue -> teal gradient. */
export function Logo({ className }: { className?: string }) {
  return (
    <svg viewBox="0 0 40 40" className={cn("size-9", className)} role="img" aria-label="Reliabilix">
      <defs>
        <linearGradient id="lg" x1="0" y1="0" x2="1" y2="1"><stop offset="0" stopColor="#3b82f6" /><stop offset="1" stopColor="#38bdf8" /></linearGradient>
        <linearGradient id="lg2" x1="0" y1="1" x2="1" y2="0"><stop offset="0" stopColor="#2dd4bf" /><stop offset="1" stopColor="#3b82f6" /></linearGradient>
      </defs>
      <path d="M8 6h13l-9 28H0z" fill="url(#lg)" transform="translate(3 0)" />
      <path d="M21 6h12l-9 28H12z" fill="url(#lg2)" opacity=".85" transform="translate(4 0)" />
    </svg>
  );
}
