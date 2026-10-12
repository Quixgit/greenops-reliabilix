"use client";

import { planAmount } from "@/lib/plan-format";
import Link from "next/link";
import { IconArrowRight, IconCard } from "@/components/ui/icons";
import { usePlan } from "@/hooks/use-admin";
import { useT } from "@/i18n/provider";
import { cn } from "@/lib/utils";

/** The plan a person is on, as the menu shows it: its name, what is left of the period, the fullest limit, and the way up. */
export function usePlanState() {
  const t = useT();
  const q = usePlan();
  const p = q.data;
  if (!p) return null;
  const days = p.period_end ? Math.ceil((Date.parse(p.period_end) - Date.now()) / 86_400_000) : null;
  const trialDays = p.trial_ends_at ? Math.ceil((Date.parse(p.trial_ends_at) - Date.now()) / 86_400_000) : null;
  const left = p.status === "trialing" ? trialDays : days;
  const paid = p.plan !== "free";
  const top = [...p.limits].filter((l) => l.limit > 0).sort((a, b) => b.used / b.limit - a.used / a.limit)[0];
  const attention = p.status === "past_due" || (paid && left !== null && left <= 7) || (!!top && top.used / top.limit >= 0.8);
  const status = p.status === "past_due" ? t("plan.card.past_due") : p.status === "canceled" ? t("plan.card.canceled") : p.status === "trialing" ? t("plan.card.trial") : "";
  const timeLeft = left === null ? "" : left < 0 ? t("plan.card.expired") : left === 0 ? t("plan.card.ends_today") : t("plan.card.days_left", { days: left });
  const action = paid && p.plan !== "enterprise" && (left === null || left > 7) && p.status === "active" ? t("plan.card.manage") : paid ? t("plan.card.renew") : t("plan.card.upgrade");
  return { p, paid, top, attention, status, timeLeft, action };
}

/** A card above the gear in the menu: the plan, what is left, and a button to the plan and billing page. */
export function PlanCard({ collapsed }: { collapsed: boolean }) {
  const t = useT();
  const s = usePlanState();
  if (!s) return null;
  const { p, top, attention, status, timeLeft, action, paid } = s;
  if (collapsed) {
    return (
      <Link href="/settings#plan" title={`${t("plan.card.title")}: ${p.name}`} aria-label={`${t("plan.card.title")}: ${p.name}`}
        className={cn("mx-auto mb-2 grid size-11 place-items-center rounded-xl transition-colors hover:bg-white/10", attention ? "text-[#f2cf8a]" : "text-slate-300")}>
        <IconCard className="size-[22px]" strokeWidth={1.7} />
      </Link>
    );
  }
  const pct = top ? Math.min(100, (top.used / top.limit) * 100) : 0;
  return (
    <Link href="/settings#plan" className="group mx-3 mb-2 block rounded-2xl bg-white/[0.06] p-3 ring-1 ring-white/10 transition-all duration-300 hover:bg-white/10 hover:ring-white/20">
      <div className="flex items-center gap-2.5">
        <span className="grid size-8 shrink-0 place-items-center rounded-lg bg-white/10 text-leaf"><IconCard className="size-[18px]" strokeWidth={1.7} /></span>
        <span className="min-w-0 flex-1 leading-tight">
          <span className="block text-[11px] font-semibold uppercase tracking-wider text-slate-400">{t("plan.card.title")}</span>
          <span className="block truncate text-[15px] font-bold text-white">{p.name}{status && <span className="ml-1.5 text-xs font-medium text-[#f2cf8a]">{status}</span>}</span>
        </span>
      </div>
      {timeLeft && <p className="mt-2 text-xs text-slate-400">{timeLeft}</p>}
      {top ? (
        <div className="mt-2.5">
          <div className="mb-1 flex justify-between text-[11px] text-slate-400"><span>{t("plan.card.limit", { used: planAmount(top.resource, top.used), limit: planAmount(top.resource, top.limit), resource: t(`plan.resource.${top.resource}`, undefined, top.resource).toLowerCase() })}</span></div>
          <div className="h-1.5 overflow-hidden rounded-full bg-white/10"><div className={cn("h-full rounded-full transition-[width] duration-700", pct >= 90 ? "bg-[#eba7b3]" : pct >= 70 ? "bg-[#f2cf8a]" : "bg-leaf")} style={{ width: `${Math.max(4, pct)}%` }} /></div>
        </div>
      ) : <p className="mt-2 text-xs text-slate-400">{paid ? t("plan.card.unlimited_hint") : t("plan.card.hint_free")}</p>}
      <span className={cn("mt-3 flex items-center justify-center gap-1.5 rounded-xl py-1.5 text-[13px] font-semibold transition-colors", paid && !attention ? "bg-white/10 text-slate-200 group-hover:bg-white/15" : "bg-leaf-deep text-white group-hover:bg-[#277a66]")}>
        {action}<IconArrowRight className="size-3.5 transition-transform duration-300 group-hover:translate-x-0.5" />
      </span>
    </Link>
  );
}
