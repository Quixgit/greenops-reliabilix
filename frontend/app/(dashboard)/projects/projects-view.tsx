"use client";

import { FolderKanban } from "lucide-react";
import { Card, CardHeader } from "@/components/ui/card";
import { EmptyState, ErrorState } from "@/components/ui/empty-state";
import { Skeleton } from "@/components/ui/skeleton";
import { ProjectForm } from "@/components/forms/project-form";
import { useProjects } from "@/hooks/use-overview";

export function ProjectsView({ canCreate }: { canCreate: boolean }) {
  const q = useProjects();
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
                <li key={p.id} className="flex items-center justify-between py-2.5"><span className="font-medium">{p.name}</span><span className="text-muted">per {p.functional_unit}</span></li>
              ))}
            </ul>
          )}
      </Card>
    </div>
  );
}
