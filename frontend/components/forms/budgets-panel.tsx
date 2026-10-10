"use client";

import { useMutation, useQueryClient } from "@tanstack/react-query";
import { Wallet } from "lucide-react";
import { useState } from "react";
import { Badge } from "@/components/ui/badge";
import { Card, CardHeader } from "@/components/ui/card";
import { EmptyState } from "@/components/ui/empty-state";
import { Skeleton } from "@/components/ui/skeleton";
import { useBudgets, useProjects } from "@/hooks/use-overview";
import { useT } from "@/i18n/provider";
import { api, apiVoid } from "@/lib/api/client";
import { budget, type BudgetStatus } from "@/lib/api/schemas";
import { formatCost } from "@/lib/formatters";
import { Field, input } from "./project-form";

const CURRENCIES = ["USD", "EUR", "GBP", "CHF", "JPY", "CAD", "AUD", "SEK", "NOK", "DKK", "PLN", "CZK", "HUF", "INR", "BRL", "SGD"];
const PERIODS = ["monthly", "quarterly", "yearly"] as const;
const TONE = { ok: "good", at_risk: "warn", exceeded: "bad" } as const;
const BAR = { ok: "bg-emerald-500", at_risk: "bg-amber-500", exceeded: "bg-red-500" } as const;

function Row({ b, scope, canEdit }: { b: BudgetStatus; scope: string; canEdit: boolean }) {
  const t = useT();
  const qc = useQueryClient();
  const del = useMutation({
    mutationFn: () => apiVoid(`finops/budgets/${b.id}`, { method: "DELETE" }),
    onSuccess: () => { void qc.invalidateQueries({ queryKey: ["budgets"] }); void qc.invalidateQueries({ queryKey: ["alerts"] }); },
  });
  const willExceed = b.state !== "exceeded" && b.projected > b.amount;
  return (
    <li className="space-y-2 py-3 text-sm">
      <div className="flex flex-wrap items-start justify-between gap-2">
        <div>
          <p className="font-medium">{scope}</p>
          <p className="text-muted">{t(`budgets.period.${b.period}`)} · {formatCost(b.amount, b.currency)}</p>
        </div>
        <Badge tone={TONE[b.state]}>{t(`budgets.state.${b.state}`)}</Badge>
      </div>
      <div className="h-2 overflow-hidden rounded-full bg-slate-100" role="progressbar" aria-valuemin={0} aria-valuemax={100} aria-valuenow={Math.min(100, Math.round(b.used_pct))}>
        <div className={`h-full rounded-full ${BAR[b.state]}`} style={{ width: `${Math.min(100, Math.max(2, b.used_pct))}%` }} />
      </div>
      <p className="flex flex-wrap justify-between gap-x-4 text-xs text-muted">
        <span>{t("budgets.spent", { spent: formatCost(b.spent, b.currency), pct: b.used_pct.toFixed(0) })}</span>
        <span className={willExceed ? "text-bad" : undefined}>{t("budgets.projected", { projected: formatCost(b.projected, b.currency) })}</span>
      </p>
      {canEdit && <button onClick={() => del.mutate()} disabled={del.isPending} className="rounded-lg px-3 py-1.5 text-xs font-semibold ring-1 ring-line hover:bg-slate-50 disabled:opacity-60">{t("budgets.delete")}</button>}
      {del.isError && <p role="alert" className="text-xs text-bad">{del.error.message}</p>}
    </li>
  );
}

function NewBudget({ currency }: { currency: string }) {
  const t = useT();
  const qc = useQueryClient();
  const projects = useProjects();
  const [scope, setScope] = useState("");
  const [amount, setAmount] = useState("");
  const [cur, setCur] = useState(CURRENCIES.includes(currency) ? currency : "USD");
  const [period, setPeriod] = useState<(typeof PERIODS)[number]>("monthly");
  const value = Number(amount.replace(/\s/g, "").replace(",", "."));
  const valid = Number.isFinite(value) && value > 0;
  const create = useMutation({
    mutationFn: () => api("finops/budgets", budget, undefined, { method: "POST", body: JSON.stringify({ project_id: scope || null, amount: value, currency: cur, period }) }),
    onSuccess: () => { setAmount(""); void qc.invalidateQueries({ queryKey: ["budgets"] }); void qc.invalidateQueries({ queryKey: ["alerts"] }); },
  });
  return (
    <form onSubmit={(e) => { e.preventDefault(); if (valid) create.mutate(); }} className="grid gap-3 sm:grid-cols-2 lg:grid-cols-[1.4fr_1fr_0.8fr_1fr_auto] lg:items-start" noValidate>
      <Field label={t("budgets.scope")}>
        <select value={scope} onChange={(e) => setScope(e.target.value)} className={input}>
          <option value="">{t("budgets.scope_org")}</option>
          {projects.data?.map((p) => <option key={p.id} value={p.id}>{p.name}</option>)}
        </select>
      </Field>
      <Field label={t("budgets.amount")} error={amount !== "" && !valid ? t("budgets.amount_invalid") : undefined}>
        <input value={amount} onChange={(e) => setAmount(e.target.value)} inputMode="decimal" placeholder="5000" className={input} />
      </Field>
      <Field label={t("budgets.currency")}>
        <select value={cur} onChange={(e) => setCur(e.target.value)} className={input}>{CURRENCIES.map((c) => <option key={c}>{c}</option>)}</select>
      </Field>
      <Field label={t("budgets.period")}>
        <select value={period} onChange={(e) => setPeriod(e.target.value as typeof period)} className={input}>
          {PERIODS.map((p) => <option key={p} value={p}>{t(`budgets.period.${p}`)}</option>)}
        </select>
      </Field>
      <button disabled={create.isPending || !valid} className="brand-gradient mt-6 rounded-xl px-4 py-2.5 text-sm font-semibold text-white disabled:opacity-60">{t("budgets.create")}</button>
      {create.isError && <p role="alert" className="text-sm text-bad sm:col-span-2 lg:col-span-5">{create.error.message}</p>}
    </form>
  );
}

/** Budgets of the organization and of its projects with the consumption of the current period. Hidden without billing access. */
export function BudgetsPanel({ canEdit, currency }: { canEdit: boolean; currency: string }) {
  const t = useT();
  const q = useBudgets();
  const projects = useProjects();
  if (q.isError) return null; // no billing access (403): the page simply has no budgets section
  const name = (id: string | null) => (id === null ? t("budgets.scope_org") : projects.data?.find((p) => p.id === id)?.name ?? t("budgets.project_unknown"));
  return (
    <Card index={1}>
      <CardHeader title={t("budgets.title")} />
      <p className="mb-3 text-sm text-muted">{t("budgets.intro")}</p>
      {canEdit && <div className="mb-4"><NewBudget currency={currency} /></div>}
      {q.isLoading ? <Skeleton className="h-24" /> : q.data!.length === 0 ? (
        <EmptyState icon={Wallet} text={t("budgets.empty")} />
      ) : <ul className="divide-y divide-line/70">{q.data!.map((b) => <Row key={b.id} b={b} scope={name(b.project_id)} canEdit={canEdit} />)}</ul>}
    </Card>
  );
}
