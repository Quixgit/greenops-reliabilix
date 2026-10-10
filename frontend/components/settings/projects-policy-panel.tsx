"use client";

import { useQuery } from "@tanstack/react-query";
import { ChevronDown, FolderKanban } from "lucide-react";
import { useState } from "react";
import { Card, CardHeader } from "@/components/ui/card";
import { EmptyState, ErrorState } from "@/components/ui/empty-state";
import { Skeleton } from "@/components/ui/skeleton";
import { Pill } from "@/components/operator/ui";
import { PolicyPanel } from "@/components/forms/policy-panel";
import { useProjects } from "@/hooks/use-overview";
import { api } from "@/lib/api/client";
import { z } from "zod";
import { cn } from "@/lib/utils";

const policy = z.object({
  allowed_regions: z.array(z.string()), alert_carbon_monthly_kg: z.number().nullable().optional(),
  ci_carbon_block_kg: z.number().nullable(), ci_cost_block: z.number().nullable(), auto_apply_types: z.array(z.string()).default([]),
});

function ProjectRow({ id, name, unit, canEdit }: { id: string; name: string; unit: string; canEdit: boolean }) {
  const [open, setOpen] = useState(false);
  const q = useQuery({ queryKey: ["policy", id], queryFn: () => api(`projects/${id}/policy`, policy) });
  const p = q.data;
  return (
    <>
      <tr className="cursor-pointer transition-colors hover:bg-sky-50/50" onClick={() => setOpen((o) => !o)}>
        <td className="border-b border-line/60 px-4 py-3 font-medium">{name}<span className="block text-xs font-normal text-muted">per {unit.replace(/_/g, " ")}</span></td>
        <td className="border-b border-line/60 px-4 py-3">
          {!p ? <span className="text-muted">…</span> : p.allowed_regions.length === 0
            ? <Pill tone="warn">not set: no region moves proposed</Pill>
            : <span className="text-sm">{p.allowed_regions.slice(0, 2).join(", ")}{p.allowed_regions.length > 2 && <span className="text-muted"> +{p.allowed_regions.length - 2}</span>}</span>}
        </td>
        <td className="hidden border-b border-line/60 px-4 py-3 md:table-cell">
          {!p ? "" : p.auto_apply_types.length ? <Pill tone="good">{p.auto_apply_types.join(", ").replace(/_/g, " ")}</Pill> : <span className="text-muted">approval always</span>}
        </td>
        <td className="hidden border-b border-line/60 px-4 py-3 tabular-nums lg:table-cell">{p?.alert_carbon_monthly_kg != null ? `${p.alert_carbon_monthly_kg} kg/mo` : <span className="text-muted">no limit</span>}</td>
        <td className="hidden border-b border-line/60 px-4 py-3 lg:table-cell">{p?.ci_carbon_block_kg != null || p?.ci_cost_block != null ? <Pill tone="info">CI gate on</Pill> : <span className="text-muted">off</span>}</td>
        <td className="border-b border-line/60 px-4 py-3 text-right"><ChevronDown className={cn("inline size-4 text-muted transition-transform", open && "rotate-180")} aria-hidden /></td>
      </tr>
      {open && (
        <tr><td colSpan={6} className="border-b border-line/60 bg-slate-50/50 p-4"><PolicyPanel projectId={id} canEdit={canEdit} /></td></tr>
      )}
    </>
  );
}

/** Every project's residency, automation, carbon limit and CI gate in one place; a row opens the full policy. */
export function ProjectsPolicyPanel({ canEdit }: { canEdit: boolean }) {
  const q = useProjects();
  return (
    <Card interactive={false}>
      <CardHeader title="Projects & policies" icon={<FolderKanban className="size-4 text-emerald-600" />} />
      <p className="-mt-2 mb-4 text-xs text-muted">Where each project may run (data residency), what the platform may apply on its own, its monthly carbon limit and the CI gate. Open a row to change it.</p>
      {q.isLoading ? <Skeleton className="h-32" /> : q.isError ? <ErrorState /> : q.data!.length === 0 ? <EmptyState icon={FolderKanban} text="No projects yet." /> : (
        <div className="overflow-hidden rounded-2xl ring-1 ring-line">
          <div className="overflow-x-auto">
            <table className="w-full min-w-[520px] border-separate border-spacing-0 text-sm">
              <thead>
                <tr className="bg-slate-50 text-left text-xs font-semibold uppercase tracking-wide text-slate-500">
                  <th className="border-b border-line px-4 py-2.5">Project</th><th className="border-b border-line px-4 py-2.5">Allowed regions</th>
                  <th className="hidden border-b border-line px-4 py-2.5 md:table-cell">Applies on its own</th>
                  <th className="hidden border-b border-line px-4 py-2.5 lg:table-cell">Carbon limit</th><th className="hidden border-b border-line px-4 py-2.5 lg:table-cell">CI gate</th><th className="border-b border-line px-4 py-2.5" />
                </tr>
              </thead>
              <tbody>{q.data!.map((p) => <ProjectRow key={p.id} id={p.id} name={p.name} unit={p.functional_unit} canEdit={canEdit} />)}</tbody>
            </table>
          </div>
        </div>
      )}
    </Card>
  );
}
