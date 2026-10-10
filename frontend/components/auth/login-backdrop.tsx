import { cn } from "@/lib/utils";

/**
 * The sign-in scene: a dark, engineered control-room look. A fine engineering grid, two telemetry traces (cost and
 * carbon), server racks with status lights, region nodes joined by flowing lines, and instrument panels with
 * monospaced labels. Decorative only: aria-hidden and without numbers that could be mistaken for data.
 */
export function LoginBackdrop() {
  return (
    <div aria-hidden className="pointer-events-none absolute inset-0 overflow-hidden bg-[#0a1420]">
      <div className="absolute inset-0 bg-[radial-gradient(ellipse_at_50%_35%,#16324a_0%,#0d1c2c_45%,#08111b_100%)]" />

      <svg viewBox="0 0 1440 900" preserveAspectRatio="xMidYMid slice" className="absolute inset-0 h-full w-full">
        <defs>
          <pattern id="ind-minor" width="40" height="40" patternUnits="userSpaceOnUse"><path d="M40 0H0V40" fill="none" stroke="#7fa6c9" strokeOpacity="0.10" /></pattern>
          <pattern id="ind-major" width="200" height="200" patternUnits="userSpaceOnUse"><path d="M200 0H0V200" fill="none" stroke="#7fa6c9" strokeOpacity="0.20" /></pattern>
          <linearGradient id="ind-cost" x1="0" y1="0" x2="1" y2="0"><stop offset="0" stopColor="#38bdf8" stopOpacity="0" /><stop offset="0.3" stopColor="#38bdf8" /><stop offset="1" stopColor="#7dd3fc" /></linearGradient>
          <linearGradient id="ind-co2" x1="0" y1="0" x2="1" y2="0"><stop offset="0" stopColor="#34d399" stopOpacity="0" /><stop offset="0.3" stopColor="#34d399" /><stop offset="1" stopColor="#86efac" /></linearGradient>
          <linearGradient id="ind-fill" x1="0" y1="0" x2="0" y2="1"><stop offset="0" stopColor="#38bdf8" stopOpacity="0.16" /><stop offset="1" stopColor="#38bdf8" stopOpacity="0" /></linearGradient>
        </defs>
        <rect width="1440" height="900" fill="url(#ind-minor)" />
        <rect width="1440" height="900" fill="url(#ind-major)" />

        {/* telemetry traces with axes ticks */}
        <g stroke="#7fa6c9" strokeOpacity="0.35">
          <path d="M60 820 H1380" />
          {Array.from({ length: 28 }, (_, i) => <path key={i} d={`M${60 + i * 49} 820 v${i % 4 === 0 ? 12 : 6}`} />)}
        </g>
        <path d="M60 740 C 260 730, 380 690, 560 650 S 900 560, 1100 520 S 1300 470, 1380 450 V820 H60Z" fill="url(#ind-fill)" />
        <path className="draw" style={{ ["--len" as string]: 1900, ["--d" as string]: "300ms" }} d="M60 740 C 260 730, 380 690, 560 650 S 900 560, 1100 520 S 1300 470, 1380 450"
          fill="none" stroke="url(#ind-cost)" strokeWidth="2" />
        <path className="draw" style={{ ["--len" as string]: 1900, ["--d" as string]: "700ms" }} d="M60 560 C 280 570, 420 610, 640 640 S 980 700, 1180 712 S 1330 724, 1380 726"
          fill="none" stroke="url(#ind-co2)" strokeWidth="2" />

        {/* region nodes and the lines that join them */}
        <g fill="none" stroke="#5eead4" strokeOpacity="0.55" strokeWidth="1.2">
          <path className="ind-flow" d="M250 250 L520 170 L860 210 L1180 150" />
          <path className="ind-flow" style={{ animationDelay: "-1.2s" }} d="M520 170 L610 340 L930 380 L1240 300" />
          <path className="ind-flow" style={{ animationDelay: "-2.1s" }} d="M610 340 L330 470 L170 400" />
        </g>
        {[[250, 250], [520, 170], [860, 210], [1180, 150], [610, 340], [930, 380], [1240, 300], [330, 470], [170, 400]].map(([x, y], i) => (
          <g key={i}>
            <circle cx={x} cy={y} r="9" fill="none" stroke="#5eead4" strokeOpacity="0.35" className="ind-pulse" style={{ animationDelay: `${i * -0.5}s` }} />
            <rect x={x - 3.5} y={y - 3.5} width="7" height="7" fill="#5eead4" />
          </g>
        ))}
      </svg>

      {/* scan line */}
      <div className="absolute inset-x-0 top-0 h-full overflow-hidden">
        <div className="ind-scan h-24 w-full bg-gradient-to-b from-transparent via-cyan-300/[0.06] to-transparent" />
      </div>

      {/* server racks at both edges */}
      <Rack className="left-[3%] top-[12%]" delay={0} />
      <Rack className="right-[3%] top-[16%]" delay={-1.4} />

      {/* instrument panels; hidden on small screens */}
      <div className="hidden xl:block">
        <Panel className="left-[9%] top-[60%]" title="Cost / day" tone="sky">
          <div className="flex h-10 items-end gap-1">{[5, 7, 6, 8, 7, 9, 8, 6].map((h, i) => <Bar key={i} h={h} i={i} c="bg-sky-400" />)}</div>
        </Panel>
        <Panel className="right-[9%] top-[62%]" title="gCO₂e / kWh" tone="emerald">
          <div className="flex h-10 items-end gap-1">{[9, 8, 8, 6, 6, 5, 4, 4].map((h, i) => <Bar key={i} h={h} i={i} c="bg-emerald-400" />)}</div>
        </Panel>
        <Panel className="left-[9%] top-[8%]" title="Region" tone="cyan">
          <p className="font-mono text-sm tracking-wide text-cyan-100">eu-north-1</p>
        </Panel>
        <Panel className="right-[9%] top-[7%]" title="SCI / unit" tone="emerald">
          <p className="font-mono text-sm tracking-wide text-emerald-100">gCO₂e per R</p>
        </Panel>
      </div>

      <div className="absolute inset-x-0 bottom-0 h-40 bg-gradient-to-t from-[#08111b] to-transparent" />
    </div>
  );
}

function Rack({ className, delay }: { className: string; delay: number }) {
  return (
    <div className={cn("ind-panel absolute hidden w-28 rounded-sm p-2 lg:block", className)}>
      <div className="space-y-1.5">
        {Array.from({ length: 9 }, (_, i) => (
          <div key={i} className="flex h-5 items-center justify-between rounded-[2px] border border-slate-500/30 bg-slate-900/60 px-1.5">
            <span className="h-px w-8 bg-slate-500/50" />
            <span className="ind-led size-1.5 rounded-full" style={{ background: i % 5 === 3 ? "#fbbf24" : "#34d399", animationDelay: `${delay - i * 0.45}s` }} />
          </div>
        ))}
      </div>
    </div>
  );
}

const TONE = { sky: "text-sky-300/90", emerald: "text-emerald-300/90", cyan: "text-cyan-300/90" } as const;

function Panel({ className, title, tone, children }: { className: string; title: string; tone: keyof typeof TONE; children: React.ReactNode }) {
  return (
    <div className={cn("ind-panel absolute w-44 rounded-sm px-3.5 py-3", className)}>
      <span className="absolute -left-px -top-px size-2 border-l border-t border-cyan-300/70" />
      <span className="absolute -bottom-px -right-px size-2 border-b border-r border-cyan-300/70" />
      <p className={cn("font-mono text-[10px] uppercase tracking-[0.18em]", TONE[tone])}>{title}</p>
      <div className="mt-2">{children}</div>
    </div>
  );
}

function Bar({ h, i, c }: { h: number; i: number; c: string }) {
  return <span className={cn("ind-bar block w-3 rounded-[1px]", c)} style={{ height: `${h * 10}%`, animationDelay: `${i * -0.5}s`, opacity: 0.85 }} />;
}
