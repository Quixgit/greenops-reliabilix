"use client";

import { planAmount } from "@/lib/plan-format";
import { SelectField } from "@/components/ui/select-field";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useEffect, useMemo, useState, type CSSProperties } from "react";
import { Card, CardHeader } from "@/components/ui/card";
import { CountUp } from "@/components/ui/count-up";
import { EmptyState, ErrorState } from "@/components/ui/empty-state";
import { IconArrowRight, IconCard, IconCheck, IconClock, IconLock, IconMinus, IconRefresh, IconShield, IconWarning } from "@/components/ui/icons";
import { Info } from "@/components/ui/info";
import { Skeleton } from "@/components/ui/skeleton";
import { useBillingHistory, usePlan } from "@/hooks/use-admin";
import { useT } from "@/i18n/provider";
import { api } from "@/lib/api/client";
import { checkoutResult, type PlanView } from "@/lib/api/schemas";
import { nf, timeAgo } from "@/lib/formatters";
import { cn } from "@/lib/utils";

const PENDING_KEY = "rlx.checkout";
const STATUS_CHIP: Record<string, string> = { active: "bg-leaf-soft text-leaf-deep", trialing: "bg-[#eef3fd] text-[#4a6fb0]", past_due: "bg-[#fdf3dc] text-[#a9741a]", canceled: "bg-[#fdecef] text-[#c25667]", none: "bg-slate-100 text-slate-500" };
const money = (a: number, c: string) => new Intl.NumberFormat(undefined, { style: "currency", currency: c, maximumFractionDigits: a % 1 ? 2 : 0 }).format(a);
const daysLeft = (iso: string | null) => (iso ? Math.ceil((Date.parse(iso) - Date.now()) / 86_400_000) : null);

type Order = { plan: string; intent: "renew" | "upgrade" | "downgrade" };

/** What the person has now, what else exists, and how to pay: renew the plan, move up, or move down. */
export function BillingTab({ canBuy }: { canBuy: boolean }) {
  const t = useT();
  const qc = useQueryClient();
  const [waiting, setWaiting] = useState<string | null>(null);
  const q = usePlan(waiting ? 4000 : false);
  const history = useBillingHistory();
  const [order, setOrder] = useState<Order | null>(null);
  const [provider, setProvider] = useState("");
  const [note, setNote] = useState<string | null>(null);

  // After the provider's page the person lands here again: keep asking until the plan they paid for is in place.
  useEffect(() => { try { setWaiting(sessionStorage.getItem(PENDING_KEY)); } catch { /* storage blocked */ } }, []);
  const p = q.data;
  useEffect(() => {
    if (!waiting || !p) return;
    if (p.plan === waiting && p.status === "active") {
      try { sessionStorage.removeItem(PENDING_KEY); } catch { /* ignore */ }
      setWaiting(null); void qc.invalidateQueries({ queryKey: ["plan-history"] });
    }
  }, [waiting, p, qc]);

  const buy = useMutation({
    mutationFn: (v: { provider: string; plan: string }) => api("plan/checkout", checkoutResult, undefined, { method: "POST", body: JSON.stringify(v) }),
    onSuccess: (r, v) => {
      void qc.invalidateQueries({ queryKey: ["plan-history"] });
      if (r.url) {
        try { sessionStorage.setItem(PENDING_KEY, v.plan); } catch { /* ignore */ }
        window.location.assign(r.url);
      } else if (r.message) { setNote(r.message); setOrder(null); }
    },
  });

  const cat = p?.catalog ?? [];
  const current = cat.find((c) => c.id === p?.plan);
  const offers = (plan: string) => (p?.providers ?? []).filter((x) => x.plans.includes(plan));
  const priceOf = (plan: string, prov?: string) => {
    const ps = prov ? (p?.providers ?? []).filter((x) => x.id === prov) : offers(plan);
    for (const x of ps) { const pr = x.prices[plan]; if (pr) return pr; }
    return null;
  };
  const intentOf = (id: string): Order["intent"] => {
    const target = cat.find((c) => c.id === id);
    if (!target || !current || id === p?.plan) return "renew";
    return target.rank > current.rank ? "upgrade" : "downgrade";
  };

  const open = (plan: string) => {
    const o = offers(plan);
    setProvider(o[0]?.id ?? "");
    setNote(null);
    setOrder({ plan, intent: intentOf(plan) });
  };

  if (q.isLoading) return <Skeleton className="h-64" />;
  if (q.isError || !p) return <ErrorState />;
  const ends = p.status === "trialing" ? p.trial_ends_at : p.period_end;
  const left = daysLeft(ends);
  const soon = left !== null && left <= 7 && left >= 0;
  const expired = left !== null && left < 0;
  const hasProviders = p.providers.length > 0;
  const currentIsFree = p.plan === "free";
  const target = order ? cat.find((c) => c.id === order.plan) : null;

  return (
    <div className="space-y-5">
      {waiting && (
        <div role="status" className="flex items-center gap-3 rounded-2xl bg-[#eef3fd] px-4 py-3 text-sm text-[#3d5a8c]">
          <IconRefresh className="size-5 animate-spin" />{t("bl.waiting", { plan: cat.find((c) => c.id === waiting)?.name ?? waiting })}
        </div>
      )}

      <Card index={0} interactive={false} className="overflow-hidden">
        <div className="grid gap-6 lg:grid-cols-[minmax(0,1fr)_auto] lg:items-center">
          <div>
            <p className="flex items-center gap-2 text-xs font-semibold uppercase tracking-wide text-slate-500">{t("bl.current")}<Info>{t("bl.info.current")}</Info></p>
            <div className="mt-2 flex flex-wrap items-center gap-3">
              <h2 className="text-3xl font-bold tracking-tight">{p.name}</h2>
              <span className={cn("rounded-full px-3 py-1 text-xs font-semibold", STATUS_CHIP[p.status])}>{t(`plan.status.${p.status}`)}</span>
            </div>
            {ends && (
              <p className="mt-2 text-sm text-slate-600">
                {p.status === "trialing" ? t("plan.trial_ends") : expired || p.status === "canceled" ? t("plan.ended") : t("plan.period_end")}: <b className="text-ink">{new Date(ends).toLocaleDateString()}</b>
                {left !== null && left >= 0 && <span className={cn("ml-2 rounded-full px-2 py-0.5 text-xs font-semibold", soon ? "bg-[#fdf3dc] text-[#a9741a]" : "bg-slate-100 text-slate-600")}>{t("bl.days_left", { count: left })}</span>}
              </p>
            )}
            {!ends && currentIsFree && <p className="mt-2 text-sm text-slate-600">{t("bl.free_note")}</p>}
            {p.status === "past_due" && <p className="mt-3 flex items-start gap-2 rounded-xl bg-[#fdf3dc] px-3 py-2 text-sm text-[#a9741a]"><IconWarning className="mt-0.5 size-4 shrink-0" />{t("plan.past_due_note")}</p>}
            {(soon || expired) && p.status !== "past_due" && !currentIsFree && <p className="mt-3 flex items-start gap-2 rounded-xl bg-[#fdf3dc] px-3 py-2 text-sm text-[#a9741a]"><IconClock className="mt-0.5 size-4 shrink-0" />{expired ? t("bl.expired_note") : t("bl.soon_note")}</p>}
          </div>
          <div className="flex flex-wrap items-center gap-3 lg:justify-end">
            {canBuy && hasProviders && !currentIsFree && (
              <button type="button" onClick={() => open(p.plan)} className="btn-leaf inline-flex items-center gap-2 rounded-xl px-5 py-3 text-sm font-semibold"><IconRefresh className="size-4" strokeWidth={2.2} />{t("bl.renew", { plan: p.name })}</button>
            )}
            <a href="#bl-plans" className="rounded-xl px-4 py-3 text-sm font-semibold text-leaf-deep ring-1 ring-line transition-colors hover:bg-leaf-soft">{currentIsFree ? t("bl.see_plans") : t("bl.change")}</a>
          </div>
        </div>
        {!canBuy && <p className="mt-4 border-t border-line/70 pt-3 text-xs text-muted">{t("plan.admin_only")}</p>}
      </Card>

      <section id="bl-plans" className="space-y-3">
        <h2 className="flex items-center gap-1.5 text-[15px] font-semibold tracking-tight">{t("bl.plans")}<Info>{t("bl.info.plans")}</Info></h2>
        <ul className="grid gap-4 md:grid-cols-2 xl:grid-cols-4">
          {cat.map((c, i) => {
            const isCurrent = c.id === p.plan;
            const intent = intentOf(c.id);
            const price = priceOf(c.id);
            const sold = offers(c.id).length > 0;
            const paid = c.id !== "free";
            return (
              <li key={c.id} className="rise" style={{ "--i": i } as CSSProperties}>
                <div className={cn("flex h-full flex-col rounded-2xl border bg-white p-5 transition-all duration-300 hover:-translate-y-1 hover:shadow-[0_14px_32px_-14px_rgba(47,143,120,0.35)]", isCurrent ? "border-leaf shadow-[0_0_0_3px_rgba(111,195,168,0.18)]" : "border-line")}>
                  <div className="flex items-center justify-between">
                    <h3 className="text-lg font-bold tracking-tight">{c.name}</h3>
                    {isCurrent && <span className="rounded-full bg-leaf px-2.5 py-0.5 text-[11px] font-bold text-white">{t("plan.current")}</span>}
                  </div>
                  <p className="mt-1 min-h-[2.5rem] text-xs leading-snug text-muted">{c.description}</p>
                  <p className="mt-3 min-h-[3.25rem] text-2xl font-bold tracking-tight tabular-nums">
                    {!paid ? t("bl.free_price") : price ? <>{money(price.amount, price.currency)}<span className="text-sm font-medium text-slate-500"> / {t("bl.period")}</span></> : c.price_monthly_usd != null ? <><span>{money(c.price_monthly_usd, "USD")}</span><span className="text-sm font-medium text-slate-500"> / {t("bl.period")}</span><span className="block text-[11px] font-normal text-muted">{t("bl.list_price")}</span></> : <span className="text-base font-semibold text-slate-600">{t("bl.contact")}</span>}
                  </p>
                  <ul className="mt-4 space-y-2 text-sm">
                    {c.limits.map((l) => (
                      <li key={l.key} className="flex items-center justify-between"><span className="text-slate-600">{t(`plan.resource.${l.resource}`, undefined, l.resource)}</span><span className="font-semibold tabular-nums">{l.limit === 0 ? "∞" : planAmount(l.resource, l.limit)}</span></li>
                    ))}
                  </ul>
                  <ul className="mt-4 grid gap-1.5 border-t border-line/70 pt-4 text-sm">
                    {c.modules.map((m) => (
                      <li key={m.key} className={cn("flex items-center gap-2", !m.enabled && "text-slate-400")}>
                        {m.enabled ? <IconCheck className="size-4 text-leaf-deep" strokeWidth={2.6} /> : <IconMinus className="size-4" />}{t(`plan.module.${m.key.replace("modules.", "")}`, undefined, m.key)}
                      </li>
                    ))}
                  </ul>
                  <div className="mt-auto pt-5">
                    {!paid ? (
                      <p className="text-xs text-muted">{isCurrent ? t("bl.free_current") : t("bl.free_after")}</p>
                    ) : canBuy && sold ? (
                      <button type="button" onClick={() => open(c.id)}
                        className={cn("inline-flex w-full items-center justify-center gap-2 rounded-xl px-4 py-2.5 text-sm font-semibold", intent === "downgrade" ? "bg-slate-100 text-slate-700 transition-colors hover:bg-slate-200" : "btn-leaf")}>
                        {intent === "renew" ? t("bl.renew", { plan: c.name }) : intent === "upgrade" ? t("bl.upgrade", { plan: c.name }) : t("bl.downgrade", { plan: c.name })}
                        <IconArrowRight className="size-4" />
                      </button>
                    ) : (
                      <p className="text-xs text-muted">{!sold ? t("bl.not_sold") : t("plan.admin_only")}</p>
                    )}
                  </div>
                </div>
              </li>
            );
          })}
        </ul>
        {!hasProviders && <p className="rounded-2xl bg-slate-50 px-4 py-3 text-sm text-slate-600">{t("plan.no_providers")}</p>}
        {note && <p role="status" className="rounded-2xl bg-leaf-soft px-4 py-3 text-sm text-leaf-deep">{note}</p>}
      </section>

      {order && target && (
        <Card interactive={false} className="form-swap border border-leaf/40">
          <CardHeader icon={<IconCard className="size-[18px] text-leaf-deep" />} title={t(`bl.order.${order.intent}`, { plan: target.name })} info={t("bl.info.order")} />
          <div className="grid gap-5 lg:grid-cols-2">
            <dl className="space-y-3 text-sm">
              <div className="flex justify-between"><dt className="text-muted">{t("bl.order.plan")}</dt><dd className="font-semibold">{target.name}</dd></div>
              <div className="flex justify-between"><dt className="text-muted">{t("bl.order.for")}</dt><dd className="font-semibold">{t(`bl.intent.${order.intent}`)}</dd></div>
              <div className="flex justify-between"><dt className="text-muted">{t("bl.order.amount")}</dt><dd className="font-semibold tabular-nums">{(() => { const pr = priceOf(order.plan, provider); return pr ? `${money(pr.amount, pr.currency)} / ${t("bl.period")}` : t("bl.price_at_checkout"); })()}</dd></div>
              {offers(order.plan).length > 1 ? (
                <label className="block text-xs font-medium text-slate-500">{t("bl.order.provider")}
                  <SelectField value={provider} onChange={(e) => setProvider(e.target.value)} className="mt-1 w-full rounded-xl border border-line bg-white px-3 py-2 text-sm">
                    {offers(order.plan).map((x) => <option key={x.id} value={x.id}>{x.name}</option>)}
                  </SelectField>
                </label>
              ) : <div className="flex justify-between"><dt className="text-muted">{t("bl.order.provider")}</dt><dd className="font-semibold">{offers(order.plan)[0]?.name}</dd></div>}
            </dl>
            <div className="space-y-3 text-sm text-slate-600">
              <p className="flex items-start gap-2"><IconShield className="mt-0.5 size-4 shrink-0 text-leaf-deep" />{t("bl.order.note")}</p>
              {order.intent === "downgrade" && <p className="flex items-start gap-2 rounded-xl bg-[#fdf3dc] px-3 py-2 text-[#a9741a]"><IconWarning className="mt-0.5 size-4 shrink-0" />{t("bl.order.downgrade_note")}</p>}
              {order.intent === "upgrade" && current && <p className="text-xs text-muted">{t("bl.order.upgrade_note")}</p>}
              <p className="flex items-start gap-2 text-xs text-muted"><IconLock className="mt-0.5 size-4 shrink-0" />{t("plan.wallets_note")}</p>
            </div>
          </div>
          <div className="mt-5 flex flex-wrap items-center gap-3 border-t border-line/70 pt-4">
            <button type="button" disabled={buy.isPending || !provider} onClick={() => buy.mutate({ provider, plan: order.plan })} className="btn-leaf inline-flex items-center gap-2 rounded-xl px-5 py-2.5 text-sm font-semibold">
              {buy.isPending ? t("bl.order.opening") : t("bl.order.pay")}<IconArrowRight className="size-4" />
            </button>
            <button type="button" onClick={() => setOrder(null)} className="rounded-xl px-4 py-2.5 text-sm font-medium text-slate-600 hover:bg-slate-100">{t("common.cancel")}</button>
            {buy.isError && <span role="alert" className="text-sm text-bad">{buy.error.message}</span>}
          </div>
        </Card>
      )}

      <div className="grid gap-5 lg:grid-cols-2">
        <Usage p={p} onUpgrade={() => document.getElementById("bl-plans")?.scrollIntoView({ behavior: "smooth" })} />
        <Card index={2} interactive={false}>
          <CardHeader icon={<IconClock className="size-[18px] text-[#5b8fdc]" />} title={t("bl.history")} info={t("bl.info.history")} />
          {history.isLoading ? <Skeleton className="h-24" /> : history.isError ? <p className="text-sm text-muted">{t("bl.history_none_access")}</p> : (history.data?.items.length ?? 0) === 0 ? (
            <EmptyState icon={IconCard} text={t("bl.history_empty")} className="py-6" />
          ) : (
            <ul className="divide-y divide-line/70">
              {history.data!.items.map((h, i) => (
                <li key={i} className="flex items-center gap-3 py-2.5 text-sm">
                  <span className={cn("size-2.5 shrink-0 rounded-full", h.kind === "applied" ? "bg-leaf" : h.kind === "requested" ? "bg-pastel-amber" : "bg-slate-300")} />
                  <span className="min-w-0 flex-1"><span className="block font-medium">{t(`bl.ev.${h.kind}`, { plan: cat.find((c) => c.id === h.plan)?.name ?? h.plan })}</span><span className="block truncate text-xs text-muted">{h.provider}{h.detail && ` · ${h.kind === "started" ? t(`bl.intent.${h.detail}`, undefined, h.detail) : h.detail}`}</span></span>
                  <span className="shrink-0 text-xs text-muted">{timeAgo(h.at)}</span>
                </li>
              ))}
            </ul>
          )}
        </Card>
      </div>
    </div>
  );
}

function Usage({ p, onUpgrade }: { p: PlanView; onUpgrade: () => void }) {
  const t = useT();
  const near = useMemo(() => p.limits.filter((l) => l.limit > 0 && l.used / l.limit >= 0.8), [p.limits]);
  return (
    <Card index={1} interactive={false}>
      <CardHeader icon={<IconShield className="size-[18px] text-leaf-deep" />} title={t("plan.usage")} info={t("info.set_plan_usage")} />
      <ul className="space-y-4">
        {p.limits.map((l, i) => {
          const pct = l.limit > 0 ? Math.min(100, (l.used / l.limit) * 100) : 0;
          return (
            <li key={l.key} className="text-sm">
              <div className="mb-1.5 flex justify-between"><span className="font-medium">{t(`plan.resource.${l.resource}`, undefined, l.resource)}</span><span className="tabular-nums text-muted"><CountUp value={l.used} format={(n) => planAmount(l.resource, Math.round(n))} /> / {l.limit === 0 ? t("plan.unlimited") : planAmount(l.resource, l.limit)}</span></div>
              <div className="h-2 overflow-hidden rounded-full bg-slate-100"><div className={cn("bar-x h-full rounded-full", pct >= 100 ? "bg-[#e08b98]" : pct >= 80 ? "bg-pastel-amber" : "bg-leaf")} style={{ width: `${l.limit === 0 ? 0 : Math.max(2, pct)}%`, "--d": `${i * 70}ms` } as CSSProperties} /></div>
            </li>
          );
        })}
      </ul>
      {p.limits.some((l) => l.resource === "tracked_spend" && l.limit > 0 && l.used > l.limit) && (
        <p className="mt-4 flex items-start gap-2 rounded-xl bg-[#fbe9ec] px-3 py-2 text-sm text-[#a23b4d]">
          <IconWarning className="mt-0.5 size-4 shrink-0" /><span>{t("bl.over_spend")} <button type="button" onClick={onUpgrade} className="font-semibold underline">{t("bl.see_plans")}</button></span>
        </p>
      )}
      {near.length > 0 && (
        <p className="mt-4 flex items-start gap-2 rounded-xl bg-[#fdf3dc] px-3 py-2 text-sm text-[#a9741a]">
          <IconWarning className="mt-0.5 size-4 shrink-0" /><span>{t("bl.near_limit")} <button type="button" onClick={onUpgrade} className="font-semibold underline">{t("bl.see_plans")}</button></span>
        </p>
      )}
    </Card>
  );
}

export { nf };
