"use client";

import { Activity as ActivityIcon, Cloud, FileText, FolderPlus, Lightbulb, RefreshCw, type LucideIcon } from "lucide-react";
import Link from "next/link";
import type { CSSProperties } from "react";
import { Card, CardHeader } from "@/components/ui/card";
import { EmptyState, ErrorState } from "@/components/ui/empty-state";
import { Skeleton } from "@/components/ui/skeleton";
import { content } from "@/content/schema";
import { useActivity } from "@/hooks/use-overview";
import { timeAgo } from "@/lib/formatters";

// Only user-facing audit actions reach this list (allowlisted server-side).
const EVENTS: Record<string, { icon: LucideIcon; tone: string; text: (target: string) => string }> = {
  "project.created": { icon: FolderPlus, tone: "bg-blue-50 text-blue-600", text: (t) => `Project created: ${t.replace(/^project:/, "")}` },
  "cloud_connection.created": { icon: Cloud, tone: "bg-emerald-50 text-emerald-600", text: () => "Cloud account connected" },
  "cloud_sync.completed": { icon: RefreshCw, tone: "bg-sky-50 text-sky-600", text: () => "Data sync completed" },
  "report.generated": { icon: FileText, tone: "bg-emerald-50 text-emerald-600", text: () => "Report generated" },
  "recommendation.created": { icon: Lightbulb, tone: "bg-blue-50 text-blue-600", text: () => "New recommendation generated" },
  "recommendation.applied": { icon: Lightbulb, tone: "bg-violet-50 text-violet-600", text: () => "Recommendation applied" },
};

/** Where an event leads: the screen that holds the thing it is about. */
function hrefOf(action: string): string {
  if (action.startsWith("cloud_")) return "/connectors";
  if (action.startsWith("recommendation")) return "/recommendations";
  if (action.startsWith("report")) return "/reports";
  if (action.startsWith("project")) return "/projects";
  return "/overview";
}

export function RecentActivity() {
  const q = useActivity();
  return (
    <Card index={8} interactive={false}>
      <CardHeader title="Recent Activity" />
      {q.isLoading ? <Skeleton className="h-24" /> : q.isError ? <ErrorState /> :
        q.data!.length === 0 ? <EmptyState icon={ActivityIcon} text={content.empty.noActivity} /> : (
          <ul className="space-y-3">
            {q.data!.map((a, i) => {
              const e = EVENTS[a.action] ?? { icon: ActivityIcon, tone: "bg-slate-100 text-slate-500", text: () => a.action };
              return (
                <li key={a.id} className="rise" style={{ "--i": i + 2 } as CSSProperties}>
                  <Link href={hrefOf(a.action)} className="group flex items-center gap-3 rounded-xl p-1.5 text-sm transition-colors hover:bg-slate-50">
                    <span className={`flex size-8 shrink-0 items-center justify-center rounded-lg transition-transform duration-300 group-hover:scale-110 ${e.tone}`}><e.icon className="size-4" /></span>
                    <span className="min-w-0 flex-1"><span className="block">{e.text(a.target)}</span><time className="text-xs text-muted" dateTime={a.at}>{timeAgo(a.at)}</time></span>
                  </Link>
                </li>
              );
            })}
          </ul>
        )}
    </Card>
  );
}
