"use client";

import { AlertTriangle, ArrowRight, Bell, CheckCircle2, Cloud, Database, Plug, Users } from "lucide-react";
import Link from "next/link";
import { useMemo } from "react";
import { Card, CardHeader } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import {
  useOperatorAudit, useOperatorConnections, useOperatorDataSources, useOperatorErrors, useOperatorStats, useOperatorSystem, useOperatorTenants,
} from "@/hooks/use-admin";
import { shortTime } from "@/lib/format-time";
import { describe } from "./audit";
import { PageTitle, Tile } from "./parts";

type Item = { key: string; text: string; href: string; icon: typeof Bell };

/** One screen for "what needs me now". Every number comes from the same endpoints as the section pages. */
export function OverviewView() {
  const tenants = useOperatorTenants();
  const errors = useOperatorErrors("open", "all");
  const stats = useOperatorStats(24);
  const conns = useOperatorConnections();
  const system = useOperatorSystem();
  const sources = useOperatorDataSources();
  const audit = useOperatorAudit(8);

  const failed24 = stats.data?.filter((s) => s.status === "failed").reduce((n, s) => n + s.count, 0);
  const attention = useMemo(() => {
    const out: Item[] = [];
    if (errors.data?.length) out.push({ key: "err", text: `${errors.data.length} open error group${errors.data.length === 1 ? "" : "s"}`, href: "/operator/errors", icon: AlertTriangle });
    if (conns.data?.length) out.push({ key: "conn", text: `${conns.data.length} cloud connection${conns.data.length === 1 ? "" : "s"} failing to sync`, href: "/operator/connections", icon: Cloud });
    if (failed24) out.push({ key: "notif", text: `${failed24} failed notification${failed24 === 1 ? "" : "s"} in 24 hours`, href: "/operator/notifications", icon: Bell });
    const missing = system.data?.integrations.filter((i) => i.status === "missing" && !/^(planned|optional)/.test(i.note)) ?? [];
    if (missing.length) out.push({ key: "int", text: `${missing.length} integration${missing.length === 1 ? "" : "s"} not set up: ${missing.map((m) => m.name).join(", ")}`, href: "/operator/data-sources", icon: Plug });
    const bad = sources.data?.filter((s) => s.state === "attention") ?? [];
    if (bad.length) out.push({ key: "src", text: `${bad.length} data source${bad.length === 1 ? "" : "s"} with failing connections`, href: "/operator/data-sources", icon: Database });
    return out;
  }, [errors.data, conns.data, failed24, system.data, sources.data]);
  const loading = [errors, stats, conns, system, sources].some((q) => q.isLoading);

  return (
    <div className="space-y-6">
      <PageTitle title="Overview" sub="The state of the platform and the people who run it. Only counts and statuses: usage data, reports, payloads and addresses of organizations are never available here." />
      <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
        <Tile index={0} label="Organizations" value={tenants.data?.length} icon={Users} color="sage" />
        <Tile index={1} label="Open errors" value={errors.data?.length} icon={AlertTriangle} color="peach" tone={errors.data?.length ? "bad" : undefined} />
        <Tile index={2} label="Failed notifications (24 h)" value={failed24} icon={Bell} color="butter" tone={failed24 ? "warn" : undefined} />
        <Tile index={3} label="Connections failing" value={conns.data?.length} icon={Cloud} color="sky" tone={conns.data?.length ? "warn" : undefined} />
      </div>

      <div className="grid gap-4 xl:grid-cols-3">
        <Card index={4} className="xl:col-span-2">
          <CardHeader title="Needs attention" />
          {loading ? <Skeleton className="h-28" /> : attention.length === 0 ? (
            <div className="flex items-center gap-3 rounded-2xl bg-emerald-50 px-4 py-5">
              <CheckCircle2 className=" size-6 text-primary" aria-hidden />
              <p className="text-sm font-medium">Nothing needs attention: no open errors, failing connections, failed deliveries or missing integrations.</p>
            </div>
          ) : (
            <ul className="space-y-2">
              {attention.map((a, i) => (
                <li key={a.key} className="rise" style={{ "--i": i } as React.CSSProperties}>
                  <Link href={a.href} className="group flex items-center gap-3 rounded-2xl bg-white/70 px-4 py-3 text-sm ring-1 ring-line transition hover:bg-white hover:shadow-sm">
                    <span className="flex size-8 shrink-0 items-center justify-center rounded-xl bg-orange-50"><a.icon className="size-4" aria-hidden /></span>
                    <span className="font-medium">{a.text}</span>
                    <ArrowRight className="ml-auto size-4 text-muted transition group-hover:translate-x-0.5" aria-hidden />
                  </Link>
                </li>
              ))}
            </ul>
          )}
        </Card>

        <Card index={5}>
          <CardHeader title="Recent operator activity" action={<Link href="/operator/audit" className="text-sm font-medium text-primary hover:underline">All</Link>} />
          {audit.isLoading ? <Skeleton className="h-28" /> : (
            <ul className="space-y-2.5 text-sm">
              {(audit.data ?? []).map((e) => {
                const d = describe(e.action);
                return (
                  <li key={e.id} className="flex items-start gap-2">
                    <span className={`mt-1.5 size-2 shrink-0 rounded-full ${d.write ? "bg-amber-400" : "bg-slate-300"}`} aria-hidden />
                    <span className="min-w-0"><span className="font-medium">{d.area}</span> <span className="text-muted">· {d.write ? "change" : "read"} · {e.operator_email || e.operator_sub}</span>
                      <span className="block text-xs text-muted">{shortTime(e.at)}</span></span>
                  </li>
                );
              })}
            </ul>
          )}
        </Card>
      </div>

      <Card index={6}>
        <CardHeader title="Platform" action={<Link href="/operator/system" className="text-sm font-medium text-primary hover:underline">System</Link>} />
        {system.isLoading ? <Skeleton className="h-16" /> : system.data && (
          <div className="flex flex-wrap items-center gap-x-8 gap-y-2 text-sm">
            <span><span className="text-muted">Version</span> <b>{system.data.version}</b></span>
            <span><span className="text-muted">Environment</span> <b>{system.data.env}</b></span>
            <span><span className="text-muted">Schema</span> <b>{system.data.expected_migration}</b></span>
            {system.data.integrations.some((i) => i.status === "stub") && <span className="rounded-full bg-violet-50 px-2.5 py-0.5 text-xs font-medium text-violet-700 ring-1 ring-violet-200">development stand-ins running</span>}
          </div>
        )}
      </Card>
    </div>
  );
}
