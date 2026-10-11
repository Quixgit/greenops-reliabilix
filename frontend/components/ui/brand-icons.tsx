import { useId } from "react";

type P = { className?: string };

/** Solid leaf with a vein, as in the design: used for carbon and carbon intensity. */
export function LeafIcon({ className }: P) {
  const id = useId();
  return (
    <svg viewBox="0 0 24 24" className={className} aria-hidden>
      <defs><linearGradient id={`${id}-l`} x1="0.1" y1="1" x2="0.9" y2="0"><stop offset="0" stopColor="#2f8f78" /><stop offset="1" stopColor="#8fd4bf" /></linearGradient></defs>
      <path d="M5.6 20.8C4.3 15.200 5.900 9.700 10.600 6.600 13.900 4.400 18.300 3.500 20.800 3.300c.3 4.200-.9 9.600-4.400 12.900-3 2.800-6.900 4.200-10.800 4.600z" fill={`url(#${id}-l)`} />
      <path d="M5.800 20.500c2.300-5.400 6.100-9.300 11.200-12" fill="none" stroke="#fff" strokeOpacity=".9" strokeWidth="1.3" strokeLinecap="round" />
    </svg>
  );
}

/** Outlined leaf, for dark surfaces. */
export function LeafOutline({ className }: P) {
  const id = useId();
  return (
    <svg viewBox="0 0 24 24" className={className} aria-hidden>
      <defs><linearGradient id={`${id}-o`} x1="0" y1="1" x2="1" y2="0"><stop offset="0" stopColor="#35c98a" /><stop offset="1" stopColor="#8af0c4" /></linearGradient></defs>
      <path d="M5.600 20.800C4.300 15.200 5.900 9.700 10.600 6.600 13.900 4.400 18.300 3.500 20.800 3.300c.3 4.200-.9 9.600-4.400 12.900-3 2.800-6.900 4.200-10.800 4.600z" fill="none" stroke={`url(#${id}-o)`} strokeWidth="1.6" strokeLinejoin="round" />
      <path d="M5.800 20.500c2.300-5.400 6.100-9.300 11.200-12" fill="none" stroke={`url(#${id}-o)`} strokeWidth="1.4" strokeLinecap="round" />
    </svg>
  );
}

/** Stack of discs (cost), blue, with the small coin of the design. */
export function StackIcon({ className }: P) {
  const id = useId();
  return (
    <svg viewBox="0 0 24 24" className={className} aria-hidden>
      <defs><linearGradient id={`${id}-s`} x1="0" y1="0" x2="1" y2="1"><stop offset="0" stopColor="#5db4ff" /><stop offset="1" stopColor="#2f6df0" /></linearGradient></defs>
      <ellipse cx="11" cy="5.200" rx="7" ry="2.700" fill={`url(#${id}-s)`} />
      <path d="M4 8.200c0 1.500 3.100 2.700 7 2.700s7-1.200 7-2.700v3c0 1.500-3.100 2.700-7 2.700s-7-1.200-7-2.700z" fill={`url(#${id}-s)`} />
      <path d="M4 13.600c0 1.500 3.100 2.700 7 2.700 1 0 2-.1 2.800-.3a4.600 4.600 0 0 0 .2 2.300c-.9.200-2 .4-3 .4-3.900 0-7-1.200-7-2.700z" fill={`url(#${id}-s)`} />
      <circle cx="18" cy="17.800" r="3.200" fill="#2f6df0" /><circle cx="18" cy="17.800" r="1.200" fill="#cfe3ff" />
    </svg>
  );
}

/** Sphere with a needle (SCI score), violet. */
export function GaugeIcon({ className }: P) {
  const id = useId();
  return (
    <svg viewBox="0 0 24 24" className={className} aria-hidden>
      <defs><radialGradient id={`${id}-g`} cx=".35" cy=".3" r=".9"><stop offset="0" stopColor="#a89bff" /><stop offset="1" stopColor="#5b44d8" /></radialGradient></defs>
      <circle cx="12" cy="12.500" r="8.500" fill={`url(#${id}-g)`} />
      <path d="M12 12.500 16.400 7.800" stroke="#fff" strokeWidth="1.800" strokeLinecap="round" />
      <circle cx="12" cy="12.500" r="1.900" fill="#fff" />
      <path d="M5.600 9.200A7.500 7.500 0 0 1 9 5.800" stroke="#fff" strokeOpacity=".5" strokeWidth="1.200" strokeLinecap="round" fill="none" />
    </svg>
  );
}
