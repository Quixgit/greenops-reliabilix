/** Calm landscape for the sign-in page: sky, a low sun, layered hills in the GreenOps blues and greens. Decorative only. */
export function LoginArt({ className }: { className?: string }) {
  return (
    <svg viewBox="0 0 800 900" preserveAspectRatio="xMidYMid slice" className={className} aria-hidden>
      <defs>
        <linearGradient id="sky" x1="0" y1="0" x2="0" y2="1">
          <stop offset="0" stopColor="#dff0ff" /><stop offset="0.55" stopColor="#eef8f3" /><stop offset="1" stopColor="#f7fbf6" />
        </linearGradient>
        <radialGradient id="sun" cx="0.5" cy="0.5" r="0.5">
          <stop offset="0" stopColor="#fff6d6" stopOpacity="0.95" /><stop offset="1" stopColor="#fff6d6" stopOpacity="0" />
        </radialGradient>
        <linearGradient id="h1" x1="0" y1="0" x2="0" y2="1"><stop offset="0" stopColor="#bfe3d0" /><stop offset="1" stopColor="#d9f0e4" /></linearGradient>
        <linearGradient id="h2" x1="0" y1="0" x2="0" y2="1"><stop offset="0" stopColor="#8ccfb0" /><stop offset="1" stopColor="#b6e3cc" /></linearGradient>
        <linearGradient id="h3" x1="0" y1="0" x2="0" y2="1"><stop offset="0" stopColor="#4fb58b" /><stop offset="1" stopColor="#7bcba8" /></linearGradient>
        <linearGradient id="h4" x1="0" y1="0" x2="0" y2="1"><stop offset="0" stopColor="#2a9a73" /><stop offset="1" stopColor="#1f7d5e" /></linearGradient>
      </defs>
      <rect width="800" height="900" fill="url(#sky)" />
      <circle cx="560" cy="330" r="190" fill="url(#sun)" />
      <circle cx="560" cy="330" r="46" fill="#fffbe9" opacity="0.9" />
      <g fill="#ffffff" opacity="0.75">
        <ellipse cx="150" cy="190" rx="90" ry="20" /><ellipse cx="215" cy="176" rx="60" ry="18" />
        <ellipse cx="640" cy="130" rx="80" ry="16" /><ellipse cx="700" cy="118" rx="46" ry="13" />
      </g>
      <path d="M0 560 L120 470 L230 540 L360 430 L500 540 L640 450 L800 560 V900 H0Z" fill="url(#h1)" />
      <path d="M0 640 Q150 540 300 620 T600 600 T800 640 V900 H0Z" fill="url(#h2)" />
      <path d="M0 720 Q200 630 400 700 T800 690 V900 H0Z" fill="url(#h3)" />
      <path d="M0 800 Q220 740 440 790 T800 780 V900 H0Z" fill="url(#h4)" />
      <g stroke="#ffffff" strokeOpacity="0.35" strokeWidth="1.2" fill="none">
        <path d="M60 760 Q200 730 340 765" /><path d="M420 745 Q560 715 720 750" />
      </g>
    </svg>
  );
}
