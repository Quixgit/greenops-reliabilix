"use client";

import { useState } from "react";
import { IconCalculator, IconCheck, IconChevronDown } from "@/components/ui/icons";
import { Card } from "@/components/ui/card";
import { ErrorState } from "@/components/ui/empty-state";
import { Skeleton } from "@/components/ui/skeleton";
import { useOperatorMethodology } from "@/hooks/use-admin";
import type { z } from "zod";
import type { methodologySet } from "@/lib/api/schemas";
import { cn } from "@/lib/utils";
import { PageTitle } from "./parts";
import { Pill } from "./ui";

type Set = z.infer<typeof methodologySet>;
const UNIT: Record<string, string> = { vcpu_hours: "kWh per vCPU-hour", memory_gb_hours: "kWh per GB-hour", storage_gb_months: "kWh per GB-month" };
const NAME: Record<string, string> = { vcpu_hours: "Compute", memory_gb_hours: "Memory", storage_gb_months: "Storage" };

function Fact({ k, children }: { k: string; children: React.ReactNode }) {
  return <div className="rounded-2xl bg-slate-50/70 px-4 py-3 ring-1 ring-line/60"><dt className="text-[11px] font-semibold uppercase tracking-wide text-slate-400">{k}</dt><dd className="mt-1 text-sm font-medium">{children}</dd></div>;
}

function Coeffs({ title, rows, unit }: { title: string; rows: [string, number][]; unit: (k: string) => string }) {
  const max = Math.max(...rows.map(([, v]) => v), 0);
  return (
    <div>
      <p className="mb-2.5 text-sm font-semibold">{title}</p>
      <ul className="space-y-2.5">
        {rows.map(([k, v]) => (
          <li key={k}>
            <div className="flex items-baseline justify-between gap-3 text-sm"><span>{NAME[k] ?? k}</span><span className="tabular-nums"><b>{v}</b> <span className="text-xs text-muted">{unit(k)}</span></span></div>
            <div className="mt-1 h-1.5 overflow-hidden rounded-full bg-slate-100"><div className="h-full rounded-full bg-pastel-blue transition-[width] duration-700" style={{ width: `${max ? Math.max(3, (v / max) * 100) : 0}%` }} /></div>
          </li>
        ))}
      </ul>
    </div>
  );
}

function Version({ m, index }: { m: Set; index: number }) {
  const [open, setOpen] = useState(m.active);
  const [more, setMore] = useState(false);
  const [copied, setCopied] = useState(false);
  const copy = () => { void navigator.clipboard?.writeText(m.checksum).then(() => { setCopied(true); window.setTimeout(() => setCopied(false), 1500); }); };
  return (
    <Card index={index} interactive={false} className={cn(m.active && "ring-1 ring-leaf/30")}>
      <button type="button" onClick={() => setOpen((o) => !o)} aria-expanded={open} className="flex w-full flex-wrap items-center gap-3 text-left">
        <span className={cn("grid size-10 shrink-0 place-items-center rounded-xl", m.active ? "bg-leaf-soft text-leaf-deep" : "bg-slate-100 text-slate-500")}><IconCalculator className="size-5" aria-hidden /></span>
        <span className="min-w-0 flex-1">
          <span className="flex flex-wrap items-center gap-2 text-lg font-bold tracking-tight">{m.version}{m.active && <Pill tone="good" pulse>active</Pill>}<Pill tone={m.status === "provisional" ? "warn" : "good"}>{m.status}</Pill></span>
          <span className="mt-0.5 block text-sm text-muted">PUE {m.pue} · {m.embodied_g_per_kwh > 0 ? `embodied ${m.embodied_g_per_kwh} g per kWh` : "embodied emissions not included"}</span>
        </span>
        <IconChevronDown className={cn("size-5 text-slate-400 transition-transform duration-300", open && "rotate-180")} aria-hidden />
      </button>
      <div className={cn("grid transition-[grid-template-rows] duration-500 ease-[var(--ease-out-soft)]", open ? "grid-rows-[1fr]" : "grid-rows-[0fr]")}>
        <div className="overflow-hidden">
          <div className="space-y-5 pt-5">
            <dl className="grid gap-3 sm:grid-cols-3">
              <Fact k="Provenance"><code className="break-words text-xs">{m.provenance}</code></Fact>
              <Fact k="Checksum (SHA-256)">
                <span className="flex items-center gap-2"><code className="truncate text-xs" title={m.checksum}>{m.checksum.slice(0, 22)}…</code>
                  <button type="button" onClick={copy} className="shrink-0 rounded-lg px-2 py-0.5 text-xs font-semibold text-primary ring-1 ring-line hover:bg-white">{copied ? <span className="inline-flex items-center gap-1"><IconCheck className="size-3" />copied</span> : "Copy"}</button></span>
              </Fact>
              <Fact k="Immutable">A change of any number is a new version, never an edit.</Fact>
            </dl>
            <div className="grid gap-8 lg:grid-cols-2">
              <Coeffs title="Measured usage" rows={Object.entries(m.energy_kwh_per_unit)} unit={(k) => UNIT[k] ?? ""} />
              <Coeffs title="Estimated from cost (low confidence)" rows={Object.entries(m.cost_based_kwh_per_usd)} unit={() => "kWh per USD"} />
            </div>
            {m.caveats.length > 0 && (
              <ul className="space-y-1.5 rounded-2xl bg-amber-50 px-4 py-3 text-sm text-slate-700 ring-1 ring-amber-100">
                {m.caveats.map((c) => <li key={c} className="flex gap-2"><span className="mt-2 size-1 shrink-0 rounded-full bg-amber-500" />{c}</li>)}
              </ul>
            )}
            <div>
              <button type="button" onClick={() => setMore((o) => !o)} aria-expanded={more} className="inline-flex items-center gap-1.5 text-sm font-semibold text-primary hover:underline">
                {more ? "Hide" : "Read"} the sources and derivations<IconChevronDown className={cn("size-4 transition-transform duration-300", more && "rotate-180")} aria-hidden />
              </button>
              <div className={cn("grid transition-[grid-template-rows] duration-500 ease-[var(--ease-out-soft)]", more ? "grid-rows-[1fr]" : "grid-rows-[0fr]")}>
                <p className="overflow-hidden whitespace-pre-line pt-0 text-sm leading-relaxed text-muted"><span className="block pt-3">{m.notes}</span></p>
              </div>
            </div>
          </div>
        </div>
      </div>
    </Card>
  );
}

/** Every coefficient set the platform has: what is active, where the numbers come from and their checksum. Read-only. */
export function MethodologyView() {
  const q = useOperatorMethodology();
  return (
    <div className="space-y-6">
      <PageTitle title="Methodology & SCI" sub="The calculation model: coefficient sets are immutable and versioned; a new set is a new version and old results keep the version they were calculated with. Customers compare versions on their own data on Carbon & Energy." />
      {q.isLoading ? <Skeleton className="h-64" /> : q.isError ? <ErrorState /> : (
        <div className="space-y-4">{[...q.data!].sort((a, b) => Number(b.active) - Number(a.active)).map((m, i) => <Version key={m.version} m={m} index={i} />)}</div>
      )}
    </div>
  );
}
