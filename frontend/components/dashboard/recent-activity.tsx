"use client";

import { Activity as ActivityIcon, Cloud, FileText, FolderPlus, Lightbulb, RefreshCw, type LucideIcon } from "lucide-react";
import { Card, CardHeader } from "@/components/ui/card";
import { EmptyState, ErrorState } from "@/components/ui/empty-state";
import { Skeleton } from "@/components/ui/skeleton";
import { content } from "@/content/schema";
import { useActivity } from "@/hooks/use-overview";
import { timeAgo } from "@/lib/formatters";

// Only user-facing audit actions reach this list (allowlisted server-side).
const EVENTS: Record<string, { icon: LucideIcon; text: (target: string) => string }> = {
  "project.created": { icon: FolderPlus, text: (t) => `Project created: ${t.replace(/^project:/, "")}` },
  "cloud_connection.created": { icon: Cloud, text: () => "Cloud account connected" },
  "cloud_sync.completed": { icon: RefreshCw, text: () => "Data sync completed" },
  "report.generated": { icon: FileText, text: () => "Report generated" },
  "recommendation.created": { icon: Lightbulb, text: () => "New recommendation available" },
  "recommendation.applied": { icon: Lightbulb, text: () => "Recommendation applied" },
};

export function RecentActivity() {
  const q = useActivity();
  return (
    <Card>
      <CardHeader title="Recent Activity" />
      {q.isLoading ? <Skeleton className="h-32" /> : q.isError ? <ErrorState /> :
        q.data!.length === 0 ? <EmptyState icon={ActivityIcon} text={content.empty.noActivity} /> : (
          <ul className="space-y-3">
            {q.data!.map((a) => {
              const e = EVENTS[a.action] ?? { icon: ActivityIcon, text: () => a.action };
              return (
                <li key={a.id} className="flex items-center gap-3 text-sm">
                  <span className="flex size-8 shrink-0 items-center justify-center rounded-full bg-carbon-soft text-carbon"><e.icon className="size-4" /></span>
                  <span className="min-w-0 flex-1 truncate">{e.text(a.target)}</span>
                  <time className="shrink-0 text-xs text-muted" dateTime={a.at}>{timeAgo(a.at)}</time>
                </li>
              );
            })}
          </ul>
        )}
    </Card>
  );
}
