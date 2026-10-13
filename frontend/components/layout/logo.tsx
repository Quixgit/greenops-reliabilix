import { cn } from "@/lib/utils";
import { Phoenix } from "./phoenix";

/** The brand mark: the phoenix of Reliabilix Systems, flat, in the colour of its surroundings (navy on light, sand on the menu). */
export function Logo({ className }: { className?: string }) {
  return <Phoenix className={cn("text-[var(--dx-navy)]", className)} />;
}
