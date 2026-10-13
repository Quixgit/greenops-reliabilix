import { cn } from "@/lib/utils";

/** Loading placeholder with a moving highlight (a still grey block feels frozen; this shows that work is happening). */
export const Skeleton = ({ className }: { className?: string }) => (
  <span aria-hidden className={cn("skeleton block rounded-lg", className)} />
);
