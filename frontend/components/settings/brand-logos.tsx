import { useId } from "react";

/** Simplified marks of the four clouds, drawn in the colours people recognise them by (no external images). */
export function BrandLogo({ id, className }: { id: "aws" | "azure" | "gcp" | "kubernetes" | "watttime" | "ember" | "entsoe"; className?: string }) {
  const g = useId();
  switch (id) {
    case "aws":
      return (
        <svg viewBox="0 0 40 40" className={className} aria-hidden>
          <rect width="40" height="40" rx="9" fill="#FF9900" />
          <text x="20" y="21" textAnchor="middle" fontFamily="Arial, sans-serif" fontWeight="700" fontSize="13" fill="#1b2330">aws</text>
          <path d="M9 27c6 4 16 4 22 0" stroke="#1b2330" strokeWidth="2" fill="none" strokeLinecap="round" />
          <path d="M28.500 25.500l3 1.500-1.200 3" stroke="#1b2330" strokeWidth="1.800" fill="none" strokeLinecap="round" strokeLinejoin="round" />
        </svg>
      );
    case "azure":
      return (
        <svg viewBox="0 0 40 40" className={className} aria-hidden>
          <rect width="40" height="40" rx="9" fill="#0b6fd6" />
          <defs><linearGradient id={g} x1="0" y1="0" x2="1" y2="1"><stop offset="0" stopColor="#7fd1ff" /><stop offset="1" stopColor="#ffffff" /></linearGradient></defs>
          <path d="M16.500 9.500L8 29.500h6.500l9-20z" fill={`url(#${g})`} />
          <path d="M20.500 15.500l11 14H17.500l5-4.200-4.500-2.500z" fill="#fff" />
        </svg>
      );
    case "gcp":
      return (
        <svg viewBox="0 0 40 40" className={className} aria-hidden>
          <rect width="40" height="40" rx="9" fill="#fff" stroke="#e3e9f3" />
          <path d="M14 27a6 6 0 0 1-.6-11.900A8 8 0 0 1 28.600 17" fill="none" stroke="#ea4335" strokeWidth="3.200" strokeLinecap="round" />
          <path d="M14 27h13a4.500 4.500 0 0 0 1.600-8.700" fill="none" stroke="#4285f4" strokeWidth="3.200" strokeLinecap="round" />
          <path d="M13.400 15.100A6 6 0 0 0 8 21" fill="none" stroke="#fbbc05" strokeWidth="3.200" strokeLinecap="round" />
          <path d="M8 21a6 6 0 0 0 6 6" fill="none" stroke="#34a853" strokeWidth="3.200" strokeLinecap="round" />
        </svg>
      );
    case "kubernetes":
      return (
        <svg viewBox="0 0 40 40" className={className} aria-hidden>
          <rect width="40" height="40" rx="9" fill="#326ce5" />
          <path d="M20 8l9.500 4.600 2.300 10.300L25.200 31h-10.400L8.200 22.900l2.300-10.300z" fill="#fff" />
          <circle cx="20" cy="20" r="5" fill="none" stroke="#326ce5" strokeWidth="1.800" />
          <path d="M20 14.500v11M15.200 17.200l9.600 5.600M24.800 17.200l-9.600 5.600" stroke="#326ce5" strokeWidth="1.600" />
        </svg>
      );
    case "watttime":
      return (
        <svg viewBox="0 0 40 40" className={className} aria-hidden>
          <rect width="40" height="40" rx="20" fill="#fff" stroke="#e3e9f3" />
          <path d="M13 13l4 14 3-9 3 9 4-14" fill="none" stroke="#1b2330" strokeWidth="2.400" strokeLinecap="round" strokeLinejoin="round" />
        </svg>
      );
    case "ember":
      return (
        <svg viewBox="0 0 40 40" className={className} aria-hidden>
          <rect width="40" height="40" rx="9" fill="#111" />
          <text x="20" y="25" textAnchor="middle" fontFamily="Georgia, serif" fontWeight="700" fontSize="12" fill="#fff">Ember</text>
        </svg>
      );
    default:
      return (
        <svg viewBox="0 0 40 40" className={className} aria-hidden>
          <rect width="40" height="40" rx="20" fill="#fff" stroke="#e3e9f3" />
          <circle cx="20" cy="20" r="9" fill="none" stroke="#1d4ed8" strokeWidth="2.600" />
          <path d="M11 20h18" stroke="#1d4ed8" strokeWidth="2.600" />
        </svg>
      );
  }
}
