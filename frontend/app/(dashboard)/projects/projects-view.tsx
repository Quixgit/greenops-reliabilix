"use client";

import { ChevronDown, FolderKanban } from "lucide-react";
import { useState } from "react";
import { Card, CardHeader } from "@/components/ui/card";
import { EmptyState, ErrorState } from "@/components/ui/empty-state";
import { Skeleton } from "@/components/ui/skeleton";
import { PolicyPanel } from "@/components/forms/policy-panel";
import { ProjectForm } from "@/components/forms/project-form";
import { useProjects } from "@/hooks/use-overview";

export function ProjectsView({ canCreate }: { canCreate: boolean }) {
  const q = useProjects();
  const [open, setOpen] = useState<string | null>(null);
  return (
    <div className="space-y-5">
      <h1 className="text-2xl font-bold tracking-tight">Projects</h1>
      {canCreate && <Card><CardHeader title="New project" /><ProjectForm /></Card>}
      <Card>
        <CardHeader title="Your projects" />
        {q.isLoading ? <Skeleton className="h-24" /> : q.isError ? <ErrorState /> : q.data!.length === 0 ?
          <EmptyState icon={FolderKanban} text="No projects yet. A project groups the cloud accounts and workloads you want to measure." /> : (
            <ul className="divide-y divide-line/70 text-sm">
              {q.data!.map((p) => (
                <li key={p.id} className="py-2.5">
                  <button type="button" onClick={() => setOpen(open === p.id ? null : p.id)} aria-expanded={open === p.id}
                    className="flex w-full items-center justify-between text-left">
                    <span className="font-medium">{p.name}</span>
                    <span className="flex items-center gap-2 text-muted">per {p.functional_unit}<ChevronDown className={`size-4 transition ${open === p.id ? "rotate-180" : ""}`} /></span>
                  </button>
                  {open === p.id && <div className="mt-3"><PolicyPanel projectId={p.id} canEdit={canCreate} /></div>}
                </li>
              ))}
            </ul>
          )}
      </Card>
    </div>
  );
}
