"use client";

import { Check, Minus } from "lucide-react";
import Link from "next/link";
import { Badge } from "@/components/ui/badge";
import { Card, CardHeader } from "@/components/ui/card";
import { ErrorState } from "@/components/ui/empty-state";
import { Skeleton } from "@/components/ui/skeleton";
import { useOperatorBillingEvents, useOperatorIntegrations, useOperatorPlans } from "@/hooks/use-admin";
import { PAYMENT_IDS } from "./data-sources";
import { shortTime } from "@/lib/format-time";
import { PageTitle, th } from "./parts";
import { Pill, type PillTone } from "./ui";

const LIMITS: [string, string][] = [
  ["limits.projects", "Projects"], ["limits.cloud_connections", "Cloud connections"], ["limits.members", "Members"], ["limits.notification_destinations", "Notification destinations"],
];
const MODULES: [string, string][] = [
  ["modules.automation", "Automation"], ["modules.kubernetes", "Kubernetes"], ["modules.reports", "Reports"],
  ["modules.recommendations", "Recommendations"], ["modules.notifications", "Notifications"], ["modules.ci_gate", "CI gate"],
];

/** The plan catalog. A plan is a preset of limits and modules; it is assigned to an organization from its page. */
/** Payment providers: set up here, next to the plans they sell. Each opens its own settings page. */
function PaymentProviders() {
  const q = useOperatorIntegrations();
  const list = (q.data ?? []).filter((i) => PAYMENT_IDS.includes(i.id));
  return (
    <Card interactive={false}>
      <CardHeader title="Payment providers" />
      <p className="-mt-2 mb-4 text-sm text-muted">Customers choose a plan on Settings → Plan and pay on the provider’s page; the provider’s callback activates it. Set up one or several: card payments worldwide (Stripe, Paddle), Ukraine in hryvnia (LiqPay, WayForPay), payment links such as Payoneer, and invoices. Apple Pay and Google Pay are offered by the providers on their payment pages.</p>
      {q.isLoading ? <Skeleton className="h-24" /> : q.isError ? <ErrorState /> : (
        <ul className="grid gap-3 sm:grid-cols-2 xl:grid-cols-3">
          {list.map((p) => (
            <li key={p.id}>
              <Link href={`/operator/data-sources/${p.id}`} className="flex h-full flex-col gap-2 rounded-2xl p-4 ring-1 ring-line transition hover:bg-slate-50">
                <span className="flex items-start justify-between gap-2">
                  <span className="font-medium">{p.name}</span>
                  <Badge tone={p.ready ? "good" : "warn"}>{p.ready ? "Set up" : "Not set up"}</Badge>
                </span>
                <span className="text-xs text-muted">{p.description.split(". ")[0]}.</span>
                <span className="mt-auto text-sm font-medium text-primary">Settings →</span>
              </Link>
            </li>
          ))}
        </ul>
      )}
    </Card>
  );
}

function BillingEvents() {
  const q = useOperatorBillingEvents();
  return (
    <Card interactive={false}>
      <CardHeader title="Payments and requests" />
      {q.isLoading ? <Skeleton className="h-20" /> : q.isError ? <ErrorState /> : q.data!.length === 0 ? (
        <p className="text-sm text-muted">No payment notification or invoice request yet. Payment providers are set up above.</p>
      ) : (
        <div className="overflow-x-auto"><table className="w-full text-sm">
          <thead><tr><th className={th}>When</th><th className={th}>Provider</th><th className={th}>What</th><th className={th}>Organization</th><th className={th}>Plan</th><th className={th}>Detail</th></tr></thead>
          <tbody className="divide-y divide-line/70">{q.data!.map((e, i) => (
            <tr key={i}>
              <td className="whitespace-nowrap py-2 pr-3 text-muted">{shortTime(e.at)}</td><td className="pr-3">{e.provider}</td>
              <td className="pr-3"><Badge tone={e.kind === "requested" ? "warn" : "good"}>{e.kind === "requested" ? "asked to pay" : "applied"}</Badge></td>
              <td className="pr-3">{e.organization_id ? <Link href={`/operator/organizations/${e.organization_id}`} className="text-primary hover:underline">{e.organization_name || e.organization_id}</Link> : "—"}</td>
              <td className="pr-3">{e.plan}</td><td className="max-w-xs break-words text-xs text-muted">{e.detail}</td>
            </tr>
          ))}</tbody>
        </table></div>
      )}
      <p className="mt-3 text-xs text-muted">“Asked to pay”: the customer chose a provider without a callback (invoice, payment link). When the money arrived, assign the plan with its period on the organization’s page.</p>
    </Card>
  );
}

const planTone = (p: string): PillTone => (p === "enterprise" ? "violet" : p === "business" ? "info" : p === "team" ? "good" : "neutral");

export function PlansView() {
  const q = useOperatorPlans();
  return (
    <div className="space-y-5">
      <PageTitle title="Plans & billing" sub="A plan is a named preset of limits and module switches. Assigning it to an organization writes those values as locked operator values with a reason; individual terms are then set on the organization’s own page." />
      <Card interactive={false}>
        <CardHeader title="Plan catalog" />
        {q.isLoading ? <Skeleton className="h-56" /> : q.isError ? <ErrorState /> : (
          <div className="overflow-hidden rounded-2xl ring-1 ring-line">
            <div className="overflow-x-auto">
              <table className="w-full min-w-[640px] border-separate border-spacing-0 text-sm">
                <thead>
                  <tr>
                    <th className="border-b border-line bg-slate-50 px-4 py-3 text-left text-xs font-semibold uppercase tracking-wide text-slate-500">Plan</th>
                    {q.data!.map((p) => (
                      <th key={p.id} className="border-b border-l border-line bg-slate-50 px-4 py-3 text-left">
                        <Pill tone={planTone(p.id)}>{p.name}</Pill>
                        <span className="mt-1 block text-[11px] font-normal normal-case text-muted">{p.organizations} organization{p.organizations === 1 ? "" : "s"}</span>
                      </th>
                    ))}
                  </tr>
                </thead>
                <tbody>
                  <tr><td colSpan={q.data!.length + 1} className="bg-slate-50/60 px-4 py-1.5 text-[11px] font-semibold uppercase tracking-wide text-slate-500">Limits</td></tr>
                  {LIMITS.map(([k, label]) => (
                    <tr key={k} className="transition-colors hover:bg-sky-50/40">
                      <td className="border-b border-line/60 px-4 py-3 text-slate-600">{label}</td>
                      {q.data!.map((p) => (
                        <td key={p.id} className="border-b border-l border-line/60 px-4 py-3 font-medium tabular-nums">
                          {p.limits[k] === undefined ? "—" : p.limits[k] === 0 ? <span className="text-leaf-deep">Unlimited</span> : p.limits[k]}
                        </td>
                      ))}
                    </tr>
                  ))}
                  <tr><td colSpan={q.data!.length + 1} className="bg-slate-50/60 px-4 py-1.5 text-[11px] font-semibold uppercase tracking-wide text-slate-500">Modules</td></tr>
                  {MODULES.map(([k, label]) => (
                    <tr key={k} className="transition-colors hover:bg-sky-50/40">
                      <td className="border-b border-line/60 px-4 py-3 text-slate-600">{label}</td>
                      {q.data!.map((p) => (
                        <td key={p.id} className="border-b border-l border-line/60 px-4 py-3">
                          {p.modules[k] === undefined ? "—" : p.modules[k]
                            ? <span className="inline-grid size-6 place-items-center rounded-full bg-leaf-soft text-leaf-deep"><Check className="size-3.5" strokeWidth={3} aria-label="on" /></span>
                            : <span className="inline-grid size-6 place-items-center rounded-full bg-slate-100 text-slate-400"><Minus className="size-3.5" aria-label="off" /></span>}
                        </td>
                      ))}
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </div>
        )}
      </Card>
      <PaymentProviders />
      <BillingEvents />
      <div className="grid gap-4 lg:grid-cols-2">
        <Card index={1}>
          <CardHeader title="How it works" />
          <ul className="list-disc space-y-1.5 pl-5 text-sm">
            <li>Customers choose a plan on their Settings → Plan page and pay at a provider you set up under Data sources; the callback activates the plan. Or assign a plan on an <Link href="/operator/organizations" className="text-primary hover:underline">organization’s page</Link>; a reason is required and is written to that organization’s audit log.</li>
            <li>Lowering a plan never deletes anything: existing projects and connections stay, only new ones are refused.</li>
            <li>Organizations that existed before plans show “free” but keep their previous unlimited values until a plan is assigned.</li>
          </ul>
        </Card>
        <Card index={2}>
          <CardHeader title="Not built yet" />
          <ul className="list-disc space-y-1.5 pl-5 text-sm text-muted">
            <li>Usage-based billing and the cost of paid APIs per organization.</li>
            <li>Invoices and tax documents (the payment providers issue receipts; the platform keeps no payment data).</li>
            <li>Plan prices in the catalog: prices live at the payment providers.</li>
          </ul>
        </Card>
      </div>
    </div>
  );
}
