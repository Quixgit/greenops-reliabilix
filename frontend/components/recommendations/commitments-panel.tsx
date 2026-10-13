"use client";

import { useEffect, useId, useState, type CSSProperties } from "react";
import { Card, CardHeader } from "@/components/ui/card";
import { CountUp } from "@/components/ui/count-up";
import { ErrorState } from "@/components/ui/empty-state";
import { IconCoin, IconWarning } from "@/components/ui/icons";
import { Skeleton } from "@/components/ui/skeleton";
import { useCommitments, type Existing as ExistingInput, type ProviderCommitment } from "@/hooks/use-commitments";
import { useT } from "@/i18n/provider";
import { formatCost } from "@/lib/formatters";
import { cn } from "@/lib/utils";

/** Small amounts (a price per hour) need their cents; formatCost rounds to whole units. */
const money2 = (v: number, currency: string) => new Intl.NumberFormat(undefined, { style: "currency", currency, maximumFractionDigits: 2 }).format(v);

const NAMES: Record<string, string> = { aws: "AWS", azure: "Azure", gcp: "Google Cloud" };

function useDebounced<T>(v: T, ms = 350) {
  const [d, setD] = useState(v);
  useEffect(() => { const id = window.setTimeout(() => setD(v), ms); return () => window.clearTimeout(id); }, [v, ms]);
  return d;
}

/** The daily spend that a commitment could cover, with the level it stays above on 80% of days drawn as a dashed line. */
function Spark({ p, currency }: { p: ProviderCommitment; currency: string }) {
  const t = useT();
  const [hover, setHover] = useState<number | null>(null);
  const id = useId();
  const w = 560, h = 120, pad = 4;
  const max = Math.max(p.peak_daily, 1) * 1.08;
  const x = (i: number) => pad + (i / Math.max(1, p.daily.length - 1)) * (w - pad * 2);
  const y = (v: number) => h - pad - (v / max) * (h - pad * 2);
  const line = p.daily.map((d, i) => `${i === 0 ? "M" : "L"}${x(i).toFixed(1)},${y(d.cost).toFixed(1)}`).join(" ");
  const area = `${line} L${x(p.daily.length - 1)},${h - pad} L${x(0)},${h - pad} Z`;
  const base = y(p.baseline_daily);
  const hd = hover !== null ? p.daily[hover] : null;
  return (
    <div className="relative">
      <svg viewBox={`0 0 ${w} ${h}`} preserveAspectRatio="none" className="h-32 w-full touch-none" role="img" aria-label={t("cm.chart_label", { name: NAMES[p.provider] ?? p.provider })}
        onPointerMove={(e) => { const r = e.currentTarget.getBoundingClientRect(); setHover(Math.max(0, Math.min(p.daily.length - 1, Math.round(((e.clientX - r.left) / r.width * w - pad) / (w - pad * 2) * (p.daily.length - 1))))); }}
        onPointerLeave={() => setHover(null)}>
        <defs><linearGradient id={id} x1="0" y1="0" x2="0" y2="1"><stop offset="0" stopColor="#5f8d6c" stopOpacity="0.35" /><stop offset="1" stopColor="#5f8d6c" stopOpacity="0.02" /></linearGradient></defs>
        <rect x={pad} y={base} width={w - pad * 2} height={h - pad - base} fill="#5f8d6c" opacity="0.10" />
        <path d={area} fill={`url(#${id})`} />
        <path d={line} fill="none" stroke="#4d7d59" strokeWidth="2" strokeLinejoin="round" strokeLinecap="round" vectorEffect="non-scaling-stroke" />
        <line x1={pad} x2={w - pad} y1={base} y2={base} stroke="#4d7d59" strokeWidth="1.2" strokeDasharray="5 4" vectorEffect="non-scaling-stroke" />
        {hd && <line x1={x(hover!)} x2={x(hover!)} y1={pad} y2={h - pad} stroke="#94a3b8" strokeWidth="1" vectorEffect="non-scaling-stroke" />}
      </svg>
      {hd && <span aria-hidden className="pointer-events-none absolute size-3 -translate-x-1/2 -translate-y-1/2 rounded-full border-2 border-white bg-[#4d7d59] shadow" style={{ left: `${(x(hover!) / w) * 100}%`, top: `${(y(hd.cost) / h) * 128}px` }} />}
      <p className="mt-1 flex items-center gap-1.5 text-[11px] text-muted"><span aria-hidden className="inline-block h-0 w-5 border-t-[1.5px] border-dashed border-[#4d7d59]" />{t("cm.baseline_note", { value: formatCost(p.baseline_daily, currency) })}</p>
      {hd && (
        <div className="pointer-events-none absolute top-1 rounded-lg bg-ink px-2.5 py-1.5 text-xs text-white shadow-lg" style={{ left: `clamp(0px, calc(${(x(hover!) / w) * 100}% - 50px), calc(100% - 110px))` }}>
          <span className="block text-[10px] opacity-70">{hd.day}</span><span className="font-semibold tabular-nums">{formatCost(hd.cost, currency)}</span>
        </div>
      )}
    </div>
  );
}

const clamp = (v: number, hi: number) => Math.min(hi, Math.max(0, v));

/** What the cloud already holds: read from it (AWS) or typed by the person, with what to do about it. */
function Existing({ p, name, currency, e, onChange }: { p: ProviderCommitment; name: string; currency: string; e: ExistingInput | undefined; onChange: (v: ExistingInput | undefined) => void }) {
  const t = useT();
  const num = (v: string) => (v === "" ? undefined : clamp(Number(v), 100));
  const set = (k: keyof ExistingInput, v: number | undefined) => { const n = { ...e, [k]: v }; onChange(n.coverage === undefined && n.utilization === undefined ? undefined : n); };
  const input = (k: keyof ExistingInput, ph: number | null, label: string) => (
    <label className="flex items-center gap-2 text-xs font-medium text-slate-500">{label}
      <span className="relative">
        <input type="number" min={0} max={100} step={1} inputMode="decimal" value={e?.[k] ?? ""} placeholder={ph === null ? "" : String(Math.round(ph))} onChange={(ev) => set(k, num(ev.target.value))} aria-label={`${name}: ${label}`}
          className="w-20 rounded-lg border border-line bg-white py-1.5 pl-2.5 pr-6 text-right text-sm font-semibold tabular-nums text-ink outline-none transition-shadow focus:shadow-[0_0_0_3px_rgba(111,195,168,0.25)]" />
        <span className="pointer-events-none absolute right-2.5 top-1/2 -translate-y-1/2 text-xs text-slate-400">%</span>
      </span>
    </label>
  );
  const tone = p.advice === "reduce" ? "bg-[#f8eceb] text-[#a8504b]" : p.advice === "hold" ? "bg-slate-100 text-slate-600" : "bg-leaf-soft text-leaf-deep";
  return (
    <div className="rounded-2xl border border-line bg-slate-50/60 p-4">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <p className="text-xs font-semibold uppercase tracking-wide text-slate-500">{t("cm.existing")}
          {p.existing_source && <span className="ml-2 rounded-full bg-white px-2 py-0.5 text-[10px] font-bold normal-case tracking-normal text-slate-500 ring-1 ring-line">{t(p.existing_source === "read" ? "cm.existing_read" : "cm.existing_entered")}</span>}</p>
        <span className={cn("rounded-full px-3 py-1 text-xs font-bold", tone)}>{t(`cm.advice.${p.advice}`)}</span>
      </div>
      <div className="mt-3 flex flex-wrap items-center gap-x-5 gap-y-2">
        {input("coverage", p.existing_coverage_pct, t("cm.existing_coverage"))}
        {input("utilization", p.existing_utilization_pct, t("cm.existing_utilization"))}
      </div>
      {p.existing_utilization_pct !== null && (
        <div className="mt-3" role="img" aria-label={`${t("cm.existing_utilization")} ${p.existing_utilization_pct}%`}>
          <div className="h-2 overflow-hidden rounded-full bg-slate-200"><div className={cn("bar-x h-full rounded-full", p.existing_utilization_pct < 90 ? "bg-pastel-amber" : "bg-leaf")} style={{ width: `${clamp(p.existing_utilization_pct, 100)}%` }} /></div>
        </div>
      )}
      <p className="mt-3 text-sm leading-relaxed text-slate-700">{p.existing_source === "" ? t("cm.advice_unknown") : t(`cm.advice_text.${p.advice}`, { pct: Math.round(p.additional_coverage_pct), util: Math.round(p.existing_utilization_pct ?? 0) })}</p>
      {p.existing_unused_monthly !== null && p.existing_unused_monthly > 0 && <p className="mt-1 text-sm font-semibold text-[#8a6420]">{t("cm.unused", { amount: formatCost(p.existing_unused_monthly, currency) })}</p>}
    </div>
  );
}

function ProviderCard({ p, i, currency, term, onDiscount, discount, def, existing, onExisting }: { p: ProviderCommitment; i: number; currency: string; term: number; onDiscount: (v: number | undefined) => void; discount: number | undefined; def: number; existing: ExistingInput | undefined; onExisting: (v: ExistingInput | undefined) => void }) {
  const t = useT();
  const name = NAMES[p.provider] ?? p.provider;
  const f = (v: number) => formatCost(v, currency);
  // Nothing to add (the target is covered, or the existing commitment is underused): zeros for a purchase that is not advised would only mislead.
  const nothingToAdd = p.existing_source !== "" && p.additional_coverage_pct === 0;
  return (
    <Card index={i} interactive={false} className="space-y-4">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <h3 className="text-[15px] font-semibold text-ink">{name}</h3>
        <label className="flex items-center gap-2 text-xs font-medium text-slate-500">{t("cm.discount")}
          <span className="relative">
            <input type="number" min={0} max={90} step={1} inputMode="decimal" value={discount ?? ""} placeholder={String(def)} onChange={(e) => onDiscount(e.target.value === "" ? undefined : Math.min(90, Math.max(0, Number(e.target.value))))}
              aria-label={`${name}: ${t("cm.discount")}`} className="w-20 rounded-lg border border-line bg-white py-1.5 pl-2.5 pr-6 text-right text-sm font-semibold tabular-nums text-ink outline-none transition-shadow focus:shadow-[0_0_0_3px_rgba(111,195,168,0.25)]" />
            <span className="pointer-events-none absolute right-2.5 top-1/2 -translate-y-1/2 text-xs text-slate-400">%</span>
          </span>
        </label>
      </div>
      {p.status === "not_enough_data" ? (
        <p className="rounded-xl bg-slate-50 px-3 py-4 text-sm text-muted">{t("cm.not_enough", { days: p.data_days })}</p>
      ) : (
        <>
          {!nothingToAdd && (
          <div className="grid gap-3 sm:grid-cols-3">
            <div><p className="text-xs font-medium text-slate-500">{t("cm.saving_month")}</p><p className="mt-0.5 text-2xl font-bold tracking-tight text-leaf-deep tabular-nums"><CountUp value={p.monthly_saving} format={(n) => f(n)} /></p></div>
            <div><p className="text-xs font-medium text-slate-500">{t("cm.saving_year")}</p><p className="mt-0.5 text-2xl font-bold tracking-tight tabular-nums"><CountUp value={p.annual_saving} format={(n) => f(n)} /></p></div>
            <div><p className="text-xs font-medium text-slate-500">{t("cm.commit_hour")}</p><p className="mt-0.5 text-2xl font-bold tracking-tight tabular-nums"><CountUp value={p.commitment_per_hour} format={(n) => `${money2(n, currency)}${t("cm.per_hour")}`} /></p></div>
          </div>
          )}
          <Existing p={p} name={name} currency={currency} e={existing} onChange={onExisting} />
          <Spark p={p} currency={currency} />
          {!nothingToAdd && (
          <div>
            <p className="mb-2 text-xs font-semibold uppercase tracking-wide text-slate-500">{t("cm.risk")}</p>
            <ul className="grid gap-2 sm:grid-cols-3">
              {p.scenarios.map((s, k) => (
                <li key={s.utilization_pct} className={cn("rise rounded-xl px-3 py-2 text-sm", s.monthly_net >= 0 ? "bg-leaf-soft/60" : "bg-[#f8eceb]")} style={{ "--i": k } as CSSProperties}>
                  <span className="block text-xs text-slate-500">{t("cm.if_used", { pct: s.utilization_pct })}</span>
                  <span className={cn("font-semibold tabular-nums", s.monthly_net >= 0 ? "text-leaf-deep" : "text-[#a8504b]")}>{s.monthly_net >= 0 ? "+" : "−"}{f(Math.abs(s.monthly_net))}{t("cm.per_month")}</span>
                </li>
              ))}
            </ul>
            <p className="mt-2 text-xs text-muted">{t("cm.break_even", { pct: p.break_even_utilization, stable: Math.round(p.stable_share_pct), term })}</p>
          </div>
          )}
          {p.possibly_committed && <p className="flex items-start gap-2 rounded-xl bg-[#faf5e3] px-3 py-2 text-xs text-[#8a6420]"><IconWarning className="mt-0.5 size-3.5 shrink-0" />{t("cm.maybe_committed")}</p>}
        </>
      )}
    </Card>
  );
}

/** A report, not a purchase: the steady part of compute and database spend and what committing to it would save. */
export function CommitmentsPanel() {
  const t = useT();
  const [lookback, setLookback] = useState(60);
  const [term, setTerm] = useState<1 | 3>(1);
  const [coverage, setCoverage] = useState(70);
  const [discounts, setDiscounts] = useState<Record<string, number | undefined>>({});
  const [existing, setExisting] = useState<Record<string, ExistingInput | undefined>>({});
  const choice = useDebounced({ lookback, term, coverage, discounts, existing });
  const q = useCommitments(choice);
  const d = q.data;
  const seg = (on: boolean) => cn("flex-1 rounded-lg px-3 py-1.5 text-sm font-semibold transition-all", on ? "bg-white text-ink shadow-sm" : "text-slate-500 hover:text-ink");
  return (
    <div className="space-y-4">
      <Card index={0} interactive={false}>
        <CardHeader icon={<IconCoin className="size-[18px] text-leaf-deep" />} title={t("cm.title")} info={t("cm.info")} />
        <p className="mb-4 max-w-3xl text-sm text-muted">{t("cm.lead")}</p>
        <div className="grid gap-5 md:grid-cols-3">
          <div>
            <p className="mb-1.5 text-xs font-semibold uppercase tracking-wide text-slate-500">{t("cm.term")}</p>
            <div className="flex gap-1 rounded-xl bg-slate-100 p-1" role="radiogroup" aria-label={t("cm.term")}>
              {([1, 3] as const).map((y) => <button key={y} type="button" role="radio" aria-checked={term === y} onClick={() => setTerm(y)} className={seg(term === y)}>{t("cm.years", { count: y })}</button>)}
            </div>
          </div>
          <div>
            <p className="mb-1.5 text-xs font-semibold uppercase tracking-wide text-slate-500">{t("cm.lookback")}</p>
            <div className="flex gap-1 rounded-xl bg-slate-100 p-1" role="radiogroup" aria-label={t("cm.lookback")}>
              {[30, 60, 90].map((n) => <button key={n} type="button" role="radio" aria-checked={lookback === n} onClick={() => setLookback(n)} className={seg(lookback === n)}>{t("cm.days", { count: n })}</button>)}
            </div>
          </div>
          <div>
            <p className="mb-1.5 flex justify-between text-xs font-semibold uppercase tracking-wide text-slate-500"><span>{t("cm.coverage")}</span><span className="tabular-nums text-ink">{coverage}%</span></p>
            <input type="range" min={10} max={100} step={5} value={coverage} onChange={(e) => setCoverage(Number(e.target.value))} aria-label={t("cm.coverage")} className="w-full accent-[#4d7d59]" />
            <p className="mt-1 text-xs text-muted">{t("cm.coverage_hint")}</p>
          </div>
        </div>
      </Card>

      {q.isLoading ? <Skeleton className="h-48" /> : q.isError ? <ErrorState /> : !d || d.providers.length === 0 ? (
        <Card interactive={false}><p className="py-6 text-center text-sm text-muted">{t("cm.empty")}</p></Card>
      ) : (
        <>
          <Card index={1} interactive={false} className="flex flex-wrap items-center justify-between gap-4 bg-gradient-to-r from-leaf-soft/70 to-white">
            <div><p className="text-xs font-semibold uppercase tracking-wide text-slate-500">{t("cm.total")}</p>
              <p className="mt-1 text-3xl font-bold tracking-tight text-leaf-deep tabular-nums"><CountUp value={d.total_annual_saving} format={(n) => formatCost(n, d.currency)} /><span className="ml-1.5 text-base font-medium text-slate-500">{t("cm.per_year")}</span></p></div>
            <p className="max-w-md text-xs text-muted">{t("cm.window", { from: d.from, to: d.to })}</p>
          </Card>
          <div className={cn("grid gap-4", d.providers.length > 1 && "xl:grid-cols-2")}>
            {d.providers.map((p, i) => (
              <ProviderCard key={p.provider} p={p} i={i + 2} currency={d.currency} term={d.term_years} def={d.default_discount_pct[p.provider] ?? 0}
                discount={discounts[p.provider]} onDiscount={(v) => setDiscounts((s) => ({ ...s, [p.provider]: v }))}
                existing={existing[p.provider]} onExisting={(v) => setExisting((s) => ({ ...s, [p.provider]: v }))} />
            ))}
          </div>
          <p className="px-1 text-xs leading-relaxed text-muted">{t("cm.caveat")}</p>
        </>
      )}
    </div>
  );
}
