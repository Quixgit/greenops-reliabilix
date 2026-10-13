import { cn } from "@/lib/utils";

type P = { className?: string };

const Stroke = ({ className, d }: P & { d: string }) => (
  <svg viewBox="0 0 24 24" className={cn(className)} fill="none" stroke="currentColor" strokeWidth="1.6" strokeLinecap="square" strokeLinejoin="miter" aria-hidden><path d={d} /></svg>
);

/** The platform's few larger marks in the DX family: a leaf, a stack, a gauge. They follow the text colour. */
export const LeafIcon = ({ className }: P) => <Stroke className={className} d="M4 20v-8l8-8h8v8l-8 8z M4 20L14 10" />;
export const LeafOutline = LeafIcon;
export const StackIcon = ({ className }: P) => <Stroke className={className} d="M5 4h14v4H5z M5 10h14v4H5z M5 16h14v4H5z" />;
export const GaugeIcon = ({ className }: P) => <Stroke className={className} d="M3 18l2.5-7.5L12 6l6.5 4.5L21 18z M12 18l4-6" />;
