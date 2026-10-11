"use client";

import { useMutation } from "@tanstack/react-query";
import { Check, CreditCard, Minus, Repeat } from "lucide-react";
import { IconGauge as Gauge } from "@/components/ui/icons";
import { useState } from "react";
import { Badge } from "@/components/ui/badge";
import { ErrorState } from "@/components/ui/empty-state";
import { Skeleton } from "@/components/ui/skeleton";
import { usePlan } from "@/hooks/use-admin";
import { useT } from "@/i18n/provider";
import { api } from "@/lib/api/client";
import { checkoutResult } from "@/lib/api/schemas";
import { shortTime } from "@/lib/format-time";
import { Panel } from "./organization-tab";

const STATUS_TONE = { active: "good", trialing: "stub", past_due: "warn", canceled: "bad", none: "neutral" } as const;
const PAID = ["team", "business"] as const;

/** The organization's plan: what it is, how much is used, and how to change it with the providers that are set up. */
export function PlanPanel({ canBuy }: { canBuy: boolean }) {
  const t = useT();
  const q = usePlan();
  const [note, setNote] = useState<string | null>(null);
  const buy = useMutation({
    mutationFn: (v: { provider: string; plan: string }) => api("plan/checkout", checkoutResult, undefined, { method: "POST", body: JSON.stringify(v) }),
    onSuccess: (r) => {
      if (r.url) window.location.assign(r.url);
      if (r.message) setNote(r.message);
    },
  });
  if (q.isLoading) return <Skeleton className="h-48" />;
  if (q.isError || !q.data) return <ErrorState />;
  const p = q.data;
  const ends = p.status === "trialing" ? p.trial_ends_at : p.period_end;
  return (
    <div className="grid gap-4 lg:grid-cols-3">
      <Panel index={0} icon={CreditCard} title={t("plan.title")} action={<Badge tone={STATUS_TONE[p.status]}>{t(`plan.status.${p.status}`)}</Badge>}>
        <p className="mt-1 text-2xl font-bold tracking-tight">{p.name}</p>
        {ends && <p className="mt-1 text-sm text-muted">{p.status === "trialing" ? t("plan.trial_ends") : p.status === "canceled" ? t("plan.ended") : t("plan.period_end")}: {shortTime(ends)}</p>}
        {p.status === "past_due" && <p className="mt-2 rounded-xl bg-amber-50 px-3 py-2 text-sm text-amber-900 ring-1 ring-amber-200">{t("plan.past_due_note")}</p>}
      </Panel>

      <Panel index={1} icon={Gauge} title={t("plan.usage")} hint={t("settings.plan.usage_hint")}>
        <ul className="space-y-3">
          {p.limits.map((l) => {
            const pct = l.limit > 0 ? Math.min(100, (l.used / l.limit) * 100) : 0;
            return (
              <li key={l.key} className="text-sm">
                <div className="mb-1 flex justify-between"><span>{t(`plan.resource.${l.resource}`, undefined, l.resource)}</span><span className="tabular-nums text-muted">{l.used} / {l.limit === 0 ? t("plan.unlimited") : l.limit}</span></div>
                <div className="h-2 overflow-hidden rounded-full bg-slate-100"><div className={`h-full rounded-full ${pct >= 100 ? "bg-bad" : pct >= 80 ? "bg-amber-500" : "bg-emerald-500"}`} style={{ width: `${l.limit === 0 ? 0 : Math.max(2, pct)}%` }} /></div>
              </li>
            );
          })}
        </ul>
        <p className="mt-4 mb-2 text-xs font-medium text-muted">{t("plan.modules")}</p>
        <ul className="grid gap-1.5 sm:grid-cols-2">
          {p.modules.map((m) => (
            <li key={m.key} className="flex items-center gap-2 text-sm">
              {m.enabled ? <Check className="size-4 text-teal-700" aria-hidden /> : <Minus className="size-4 text-slate-400" aria-hidden />}
              <span className={m.enabled ? "" : "text-muted"}>{t(`plan.module.${m.key.replace("modules.", "")}`, undefined, m.key)}</span>
            </li>
          ))}
        </ul>
      </Panel>

      <Panel index={2} icon={Repeat} title={t("plan.change")} hint={t("settings.plan.change_hint")}>
        {p.providers.length === 0 ? <p className="text-sm text-muted">{t("plan.no_providers")}</p> : (
          <div className="space-y-4">
            {p.providers.map((pr) => (
              <div key={pr.id}>
                <p className="mb-2 text-sm font-medium">{pr.name}</p>
                <div className="flex flex-wrap gap-2">
                  {PAID.filter((pl) => pr.plans.includes(pl)).map((pl) => (
                    <button key={pl} type="button" disabled={!canBuy || buy.isPending || p.plan === pl} onClick={() => { setNote(null); buy.mutate({ provider: pr.id, plan: pl }); }}
                      className="rounded-xl px-4 py-2 text-sm font-semibold ring-1 ring-line hover:bg-slate-50 disabled:opacity-50">
                      {p.plan === pl ? t("plan.current") : t("plan.choose", { plan: t(`plan.name.${pl}`, undefined, pl) })}
                    </button>
                  ))}
                </div>
              </div>
            ))}
            {!canBuy && <p className="text-xs text-muted">{t("plan.admin_only")}</p>}
            <p className="text-xs text-muted">{t("plan.wallets_note")}</p>
          </div>
        )}
        {note && <p role="status" className="mt-3 rounded-xl bg-emerald-50 px-3 py-2 text-sm text-emerald-900 ring-1 ring-emerald-200">{note}</p>}
        {buy.isError && <p role="alert" className="mt-3 text-sm text-bad">{buy.error.message}</p>}
      </Panel>
    </div>
  );
}
