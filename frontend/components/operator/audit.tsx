"use client";

import { IconPencil, IconEye, IconHistory as History, IconUsers } from "@/components/ui/icons";
import { useMemo, useState } from "react";
import { ErrorState } from "@/components/ui/empty-state";
import { Skeleton } from "@/components/ui/skeleton";
import { useOperatorAudit } from "@/hooks/use-admin";
import type { OperatorAuditEntry } from "@/lib/api/schemas";
import { shortTime } from "@/lib/format-time";
import { PageTitle, Tile } from "./parts";
import { Avatar, DataTable, Pill, SearchBox, Segmented, Select } from "./ui";

const AREAS: Record<string, string> = {
  me: "Signed in", tenants: "Organizations", notifications: "Notifications", connections: "Connections", system: "System", errors: "Errors",
  integrations: "Data sources", settings: "Platform settings", audit: "Audit log", "data-sources": "Data sources", jobs: "Jobs", operators: "Access",
  aggregation: "Aggregation", metrics: "Metrics", announcements: "Announcements", plans: "Plans", usage: "Usage", methodology: "Methodology", carbon: "Carbon",
  "api-usage": "API usage", billing: "Billing",
};

/** "POST /api/v1/operator/errors/<id>/resolve" -> { write: true, area: "Errors", what: "errors/<id>/resolve" } */
export function describe(action: string): { method: string; write: boolean; area: string; path: string } {
  const [method = "", rest = ""] = action.split(" ", 2);
  const path = rest.replace(/^\/api\/v1\/operator\/?/, "").replace(/^\//, "");
  const head = path.split(/[/?]/)[0];
  return { method, write: method !== "GET", area: AREAS[head] ?? (head || "Console"), path: path || "/" };
}

export function AuditView() {
  const [limit, setLimit] = useState("100");
  const [kind, setKind] = useState("all");
  const [q, setQ] = useState("");
  const [area, setArea] = useState("all");
  const log = useOperatorAudit(Number(limit));
  const all = useMemo(() => log.data ?? [], [log.data]);
  const writes = all.filter((e) => describe(e.action).write).length;
  const operators = new Set(all.map((e) => e.operator_email || e.operator_sub)).size;
  const rows = useMemo(() => {
    const n = q.trim().toLowerCase();
    return all.filter((e: OperatorAuditEntry) => {
      const d = describe(e.action);
      return (kind === "all" || (kind === "changes") === d.write) && (area === "all" || d.area === area) && (!n || `${e.operator_email} ${e.operator_sub} ${d.area} ${d.path}`.toLowerCase().includes(n));
    });
  }, [all, kind, q, area]);
  const areas = useMemo(() => Array.from(new Set(all.map((e) => describe(e.action).area))).sort(), [all]);

  return (
    <div className="space-y-6">
      <PageTitle title="Audit log" sub="Every read and every change made in this console, by operator. The log is append-only: this page cannot edit or delete a line. Changes to an organization are also written to that organization’s own audit log." />
      <div className="grid gap-3 sm:grid-cols-3">
        <Tile index={0} label="Entries shown" value={log.isLoading ? undefined : all.length} icon={History} color="sage" />
        <Tile index={1} label="Changes" value={log.isLoading ? undefined : writes} icon={IconPencil} color="butter" note="writes, not reads" />
        <Tile index={2} label="Operators" value={log.isLoading ? undefined : operators} icon={IconUsers} color="lavender" />
      </div>
      <div className="flex flex-wrap items-center gap-3">
        <SearchBox value={q} onChange={setQ} placeholder="Search operator, area or request…" className="w-full sm:w-80" />
        <Segmented label="Kind" value={kind} onChange={setKind} options={[{ value: "all", label: "All", count: all.length }, { value: "changes", label: "Changes", count: writes }, { value: "reads", label: "Reads", count: all.length - writes }]} />
        <Select label="Area" className="w-44" value={area} onChange={setArea} options={[{ value: "all", label: "All areas" }, ...areas.map((a) => ({ value: a, label: a }))]} />
        <Select label="Lines" className="ml-auto w-36" value={limit} onChange={setLimit} options={[50, 100, 250, 500].map((n) => ({ value: String(n), label: `Last ${n}` }))} />
      </div>
      {log.isLoading ? <Skeleton className="h-48" /> : log.isError ? <ErrorState /> : (
        <DataTable dense pageSize={25} rowKey={(e) => String(e.id)} rows={rows} empty="No entries match." initialSort={{ key: "when", dir: "desc" }}
          footer={`${rows.length} of ${all.length} entries`}
          columns={[
            { key: "when", header: "When", sort: (a, b) => a.at.localeCompare(b.at), render: (e) => <span className="whitespace-nowrap text-muted" title={e.at}>{shortTime(e.at)}</span> },
            { key: "op", header: "Operator", sort: (a, b) => (a.operator_email || a.operator_sub).localeCompare(b.operator_email || b.operator_sub),
              render: (e) => <span className="flex items-center gap-2.5"><Avatar name={e.operator_email || e.operator_sub} size="sm" /><span>{e.operator_email || e.operator_sub}{e.role && <span className="ml-2 text-xs text-muted">{e.role}</span>}</span></span> },
            { key: "area", header: "Area", sort: (a, b) => describe(a.action).area.localeCompare(describe(b.action).area), render: (e) => describe(e.action).area },
            { key: "kind", header: "Kind", sort: (a, b) => Number(describe(a.action).write) - Number(describe(b.action).write),
              render: (e) => (describe(e.action).write ? <Pill tone="warn"><IconPencil className="mr-0.5 size-3" aria-hidden />change</Pill> : <Pill tone="neutral"><IconEye className="mr-0.5 size-3" aria-hidden />read</Pill>) },
            { key: "req", header: "Request", hideBelow: "md", render: (e) => { const d = describe(e.action); return <span className="break-all font-mono text-xs text-slate-600">{d.method} {d.path}</span>; } },
          ]} />
      )}
    </div>
  );
}
