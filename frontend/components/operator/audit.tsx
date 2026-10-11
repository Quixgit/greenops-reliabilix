"use client";

import { IconHistory as History } from "@/components/ui/icons";
import { useMemo, useState } from "react";
import { Badge } from "@/components/ui/badge";
import { Card } from "@/components/ui/card";
import { EmptyState, ErrorState } from "@/components/ui/empty-state";
import { Skeleton } from "@/components/ui/skeleton";
import { useOperatorAudit } from "@/hooks/use-admin";
import type { OperatorAuditEntry } from "@/lib/api/schemas";
import { shortTime } from "@/lib/format-time";
import { PageTitle, selectCls } from "./parts";
import { Check_, DataTable, Pill } from "./ui";

const AREAS: Record<string, string> = {
  me: "Signed in", tenants: "Organizations", notifications: "Notifications", connections: "Connections", system: "System", errors: "Errors",
  integrations: "Data sources", settings: "Platform settings", audit: "Audit log", "data-sources": "Data sources", jobs: "Jobs", operators: "Access",
};

/** "POST /api/v1/operator/errors/<id>/resolve" -> { write: true, area: "Errors", what: "errors/<id>/resolve" } */
export function describe(action: string): { method: string; write: boolean; area: string; path: string } {
  const [method = "", rest = ""] = action.split(" ", 2);
  const path = rest.replace(/^\/api\/v1\/operator\/?/, "").replace(/^\//, "");
  const head = path.split(/[/?]/)[0];
  return { method, write: method !== "GET", area: AREAS[head] ?? (head || "Console"), path: path || "/" };
}

export function AuditView() {
  const [limit, setLimit] = useState(100);
  const [onlyChanges, setOnlyChanges] = useState(false);
  const log = useOperatorAudit(limit);
  const rows = useMemo(() => {
    return (log.data ?? []).filter((e: OperatorAuditEntry) => !onlyChanges || describe(e.action).write);
  }, [log.data, onlyChanges]);

  return (
    <div className="space-y-5">
      <PageTitle title="Audit log" sub="Every read and every change made in this console, by operator. The log is append-only: this page cannot edit or delete a line. Changes to an organization are also written to that organization’s own audit log." />
      <Card interactive={false}>
        <div className="mb-4 flex flex-wrap items-center gap-2">
          <Check_ label="Changes only" checked={onlyChanges} onChange={setOnlyChanges}>Changes only</Check_>
          <select aria-label="Lines" value={limit} onChange={(e) => setLimit(Number(e.target.value))} className={`${selectCls} ml-auto`}>
            {[50, 100, 250, 500].map((n) => <option key={n} value={n}>Last {n}</option>)}
          </select>
        </div>
        {log.isLoading ? <Skeleton className="h-48" /> : log.isError ? <ErrorState /> : (
          <DataTable dense rowKey={(e) => String(e.id)} rows={rows} empty="No changes in this range." initialSort={{ key: "when", dir: "desc" }}
            columns={[
              { key: "when", header: "When", sort: (a, b) => a.at.localeCompare(b.at), render: (e) => <span className="whitespace-nowrap text-muted" title={e.at}>{shortTime(e.at)}</span> },
              { key: "op", header: "Operator", sort: (a, b) => (a.operator_email || a.operator_sub).localeCompare(b.operator_email || b.operator_sub),
                render: (e) => <span>{e.operator_email || e.operator_sub}{e.role && <span className="ml-2 text-xs text-muted">{e.role}</span>}</span> },
              { key: "area", header: "Area", sort: (a, b) => describe(a.action).area.localeCompare(describe(b.action).area), render: (e) => describe(e.action).area },
              { key: "kind", header: "Kind", sort: (a, b) => Number(describe(a.action).write) - Number(describe(b.action).write),
                render: (e) => (describe(e.action).write ? <Pill tone="warn">change</Pill> : <Pill tone="neutral">read</Pill>) },
              { key: "req", header: "Request", hideBelow: "md", render: (e) => { const d = describe(e.action); return <span className="break-all font-mono text-xs text-slate-600">{d.method} {d.path}</span>; } },
            ]} />
        )}
      </Card>
    </div>
  );
}
