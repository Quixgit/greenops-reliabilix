"use client";

import type { AnyIcon as LucideIcon } from "@/components/ui/icons";
import { IconBulb, IconCard, IconChevronDown, IconCloud, IconFolder, IconRefresh, IconReport, IconTrash, IconTrend as ActivityIcon } from "@/components/ui/icons";
import Link from "next/link";
import { useMemo, useState, type CSSProperties } from "react";
import { Card } from "@/components/ui/card";
import { Glyph } from "@/components/ui/glyphs";
import { Info } from "@/components/ui/info";
import { ErrorState } from "@/components/ui/empty-state";
import { Skeleton } from "@/components/ui/skeleton";
import { useActivity } from "@/hooks/use-overview";
import { useT } from "@/i18n/provider";
import { getFormatLocale } from "@/lib/formatters";
import type { Activity } from "@/lib/api/schemas";
import { cn } from "@/lib/utils";

type Ctx = { t: ReturnType<typeof useT> };
type View = { icon: LucideIcon; tone: string; title: string; detail?: string; href: string };

const money = (v: string) => { const n = Number(v); return Number.isFinite(n) ? n.toLocaleString(getFormatLocale()) : v; };

/** What an audit event says to a person: a title, the one fact that matters and where it leads. Only user-facing events reach here. */
function describe(a: Activity, { t }: Ctx): View {
  const d = a.detail ?? {};
  switch (a.action) {
    case "project.created": return { icon: IconFolder, tone: "bg-sky-50 text-[#3f78b8]", title: t("act.project_created"), detail: d.name, href: "/projects" };
    case "project.deleted": return { icon: IconTrash, tone: "bg-rose-50 text-[#b9566a]", title: t("act.project_deleted"), href: "/projects" };
    case "project.policy_updated": return { icon: IconFolder, tone: "bg-slate-100 text-slate-600", title: t("act.project_policy"), href: "/settings#policies" };
    case "cloud_connection.created": return { icon: IconCloud, tone: "bg-leaf-soft text-leaf-deep", title: t("act.cloud_connected"), detail: d.provider?.toUpperCase(), href: "/connectors" };
    case "cloud_sync.completed": return { icon: IconRefresh, tone: "bg-sky-50 text-[#3f78b8]", title: t("act.sync"), detail: d.records ? t("act.sync_detail", { records: Number(d.records).toLocaleString(getFormatLocale()) }) : undefined, href: "/connectors" };
    case "report.generated": return { icon: IconReport, tone: "bg-leaf-soft text-leaf-deep", title: d.kind ? t("act.report", { kind: t(`rp.kind.${d.kind}`, undefined, d.kind) }) : t("act.report_plain"), detail: d.format?.toUpperCase(), href: "/reports" };
    case "recommendation.created": return { icon: IconBulb, tone: "bg-amber-50 text-[#8a6218]", title: t("act.rec_created"), detail: d.type ? t(`act.type.${d.type}`, undefined, d.type) : undefined, href: "/recommendations" };
    case "recommendation.approved": return { icon: IconBulb, tone: "bg-violet-50 text-violet-600", title: t("act.rec_approved"), href: "/recommendations" };
    case "recommendation.applied": return { icon: IconBulb, tone: "bg-violet-50 text-violet-600", title: t("act.rec_applied"), href: "/recommendations" };
    case "budget.created": return { icon: IconCard, tone: "bg-amber-50 text-[#8a6218]", title: t("act.budget"), detail: d.amount ? t("act.budget_detail", { amount: money(d.amount), period: d.period ?? "" }) : undefined, href: "/settings#policies" };
    default: return { icon: ActivityIcon, tone: "bg-slate-100 text-slate-500", title: a.action, href: "/overview" };
  }
}

const dayKey = (iso: string) => new Date(iso).toLocaleDateString("en-CA"); // the viewer's own calendar day, as YYYY-MM-DD

function dayLabel(key: string, t: Ctx["t"]) {
  const today = new Date().toLocaleDateString("en-CA");
  const yesterday = new Date(Date.now() - 86_400_000).toLocaleDateString("en-CA");
  if (key === today) return t("act.today");
  if (key === yesterday) return t("act.yesterday");
  return new Date(`${key}T12:00:00`).toLocaleDateString(getFormatLocale(), { weekday: "short", day: "numeric", month: "short" });
}

const FIRST = 5;

/** What happened in the organization, newest first and grouped by day, as a timeline: what, the one fact that matters, and when. */
export function RecentActivity() {
  const t = useT();
  const q = useActivity();
  const [all, setAll] = useState(false);
  // consecutive syncs that read nothing are one line: a sync every few minutes must not push the real events out of view
  const rows = useMemo(() => {
    const out: (Activity & { merged?: number })[] = [];
    for (const a of q.data ?? []) {
      const quiet = a.action === "cloud_sync.completed" && !Number(a.detail?.records ?? 0);
      const prev = out[out.length - 1];
      if (quiet && prev && prev.action === "cloud_sync.completed" && !Number(prev.detail?.records ?? 0) && dayKey(prev.at) === dayKey(a.at)) prev.merged = (prev.merged ?? 1) + 1;
      else out.push({ ...a });
    }
    return out;
  }, [q.data]);
  const shown = all ? rows : rows.slice(0, FIRST);
  const groups = useMemo(() => {
    const m = new Map<string, (Activity & { merged?: number })[]>();
    for (const a of shown) { const k = dayKey(a.at); m.set(k, [...(m.get(k) ?? []), a]); }
    return [...m.entries()];
  }, [shown]);
  const time = (iso: string) => new Date(iso).toLocaleTimeString(getFormatLocale(), { hour: "2-digit", minute: "2-digit" });
  let n = 0;
  return (
    <Card index={8} interactive={false} className="p-4">
      <h2 className="mb-2 flex items-center gap-2 text-[15px] font-semibold tracking-tight text-ink"><Glyph name="pulse" className="size-[18px] text-[#2f8f78]" />{t("act.title")}<Info>{t("info.activity")}</Info></h2>
      {q.isLoading ? <Skeleton className="h-32" /> : q.isError ? <ErrorState /> : rows.length === 0 ? (
        <div className="flex flex-col items-center gap-2 py-6 text-center">
          <span className="grid size-12 place-items-center rounded-full bg-slate-100 text-slate-400"><ActivityIcon className="size-6" /></span>
          <p className="max-w-[16rem] text-sm text-muted">{t("act.empty")}</p>
        </div>
      ) : (
        <div className="space-y-2.5">
          {groups.map(([key, items]) => (
            <section key={key} aria-label={dayLabel(key, t)}>
              <h3 className="mb-1.5 text-[11px] font-semibold uppercase tracking-wider text-slate-400">{dayLabel(key, t)}</h3>
              <ol className="relative space-y-0.5 before:absolute before:bottom-3 before:left-[14px] before:top-3 before:w-px before:bg-line">
                {items.map((a) => {
                  const v = describe(a, { t });
                  if (a.merged) v.detail = t("ov3.activity.syncs", { count: a.merged });
                  const i = n++;
                  return (
                    <li key={a.id} className="rise relative" style={{ "--i": Math.min(i, 8) } as CSSProperties}>
                      <Link href={v.href} className="group flex items-start gap-2.5 rounded-lg px-1 py-1 transition-colors hover:bg-slate-50">
                        <span className={cn("relative z-10 grid size-[26px] shrink-0 place-items-center rounded-full ring-4 ring-white transition-transform duration-300 group-hover:scale-110", v.tone)}><v.icon className="size-[15px]" /></span>
                        <span className="min-w-0 flex-1 pt-0.5">
                          <span className="block truncate text-[13px] font-semibold leading-snug text-ink">{v.title}</span>
                          {v.detail && <span className="block truncate text-xs text-muted">{v.detail}</span>}
                        </span>
                        <time dateTime={a.at} title={new Date(a.at).toLocaleString(getFormatLocale())} className="shrink-0 pt-1 text-[11px] tabular-nums text-slate-400">{time(a.at)}</time>
                      </Link>
                    </li>
                  );
                })}
              </ol>
            </section>
          ))}
          {rows.length > FIRST && (
            <button type="button" onClick={() => setAll((o) => !o)} aria-expanded={all} className="mx-auto flex items-center gap-1.5 text-xs font-semibold text-leaf-deep hover:underline">
              {all ? t("act.show_less") : t("act.show_all", { n: rows.length })}<IconChevronDown className={cn("size-3.5 transition-transform duration-300", all && "rotate-180")} aria-hidden />
            </button>
          )}
        </div>
      )}
    </Card>
  );
}
