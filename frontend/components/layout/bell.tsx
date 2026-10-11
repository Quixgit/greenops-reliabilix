"use client";

import * as DM from "@radix-ui/react-dropdown-menu";
import Link from "next/link";
import { useEffect, useRef, useState } from "react";
import { DropdownPanel, DropdownRoot } from "@/components/ui/dropdown";
import { IconBell, IconCard, IconSparkles, IconCheck, IconCluster, IconFlow, IconLeaf, IconReport, IconWarning, type IconType } from "@/components/ui/icons";
import { Switch } from "@/components/operator/ui";
import { eventArea, useInbox, useMarkInboxRead, type InboxItem } from "@/hooks/use-inbox";
import { useAlerts } from "@/hooks/use-overview";
import { useT } from "@/i18n/provider";
import { timeAgo } from "@/lib/formatters";
import { playChime, readSoundPref, writeSoundPref } from "@/lib/chime";
import { cn } from "@/lib/utils";

const AREA_ICON: Record<string, IconType> = { billing: IconCard, reports: IconReport, automation: IconFlow, recommendations: IconLeaf, connections: IconCluster, platform: IconSparkles, other: IconBell };
const SEV_DOT = { critical: "bg-[#e08b98]", warning: "bg-pastel-amber", info: "bg-pastel-blue" } as const;

type Row = { key: string; title: string; body: string; severity: "info" | "warning" | "critical"; at: string; href: string; unread: boolean; area: string };

/** The bell: platform messages and open alerts in one list, an honest unread count, a chime when something new arrives. */
export function NotificationBell() {
  const t = useT();
  const inbox = useInbox();
  const alerts = useAlerts("new");
  const read = useMarkInboxRead();
  const [open, setOpen] = useState(false);
  const [own, setOwn] = useState<"on" | "off" | null>(null);
  const [ring, setRing] = useState(false);
  const last = useRef<number | null>(null);

  useEffect(() => { setOwn(readSoundPref()); }, []);
  const soundOn = (own ?? (inbox.data?.sound ? "on" : "off")) === "on";

  const rows: Row[] = [
    ...(inbox.data?.items ?? []).map((i: InboxItem): Row => ({ key: i.id, title: i.title, body: i.body, severity: i.severity, at: i.created_at, href: i.path || "/alerts", unread: i.unread, area: eventArea(i.type) })),
    ...(alerts.data ?? []).map((a): Row => ({ key: `a-${a.id}`, title: a.title, body: a.detail, severity: a.severity, at: a.last_seen_at, href: "/alerts#alerts", unread: true, area: a.kind === "budget" ? "billing" : "connections" })),
  ].sort((a, z) => Date.parse(z.at) - Date.parse(a.at));
  const unread = (inbox.data?.unread ?? 0) + (alerts.data?.length ?? 0);

  // A new item since the last look: the bell rings, and a chime plays if the person (or the organization) wants it.
  useEffect(() => {
    if (last.current !== null && unread > last.current) {
      setRing(true);
      window.setTimeout(() => setRing(false), 1200);
      if (soundOn) playChime();
    }
    last.current = unread;
  }, [unread, soundOn]);

  const markAll = () => read.mutate();
  return (
    <DropdownRoot open={open} onOpenChange={setOpen}>
      <DM.Trigger aria-label={unread ? t("bell.aria_unread", { count: unread }) : t("header.notifications")}
        className="group relative grid size-10 place-items-center rounded-xl border border-line/80 bg-white text-slate-600 shadow-[0_1px_2px_rgba(16,24,40,0.04)] outline-none transition-all duration-200 hover:border-slate-300 hover:text-ink data-[state=open]:border-leaf data-[state=open]:shadow-[0_0_0_3px_rgba(111,195,168,0.22)]">
        <IconBell className={cn("size-[20px] origin-top transition-transform duration-300 group-hover:rotate-[-8deg]", ring && "bell-ring")} />
        {unread > 0 && <span className="absolute -right-1 -top-1 grid min-w-[18px] place-items-center rounded-full bg-[#e08b98] px-1 text-[10px] font-bold leading-[18px] text-white ring-2 ring-white">{unread > 9 ? "9+" : unread}</span>}
      </DM.Trigger>
      <DropdownPanel align="end">
        <div className="w-[380px] max-w-[92vw]">
          <div className="flex items-center justify-between px-3 pb-2 pt-2.5">
            <p className="text-sm font-semibold">{t("header.notifications")}</p>
            <button type="button" onClick={markAll} disabled={(inbox.data?.unread ?? 0) === 0 || read.isPending}
              className="inline-flex items-center gap-1 text-xs font-semibold text-leaf-deep hover:underline disabled:cursor-default disabled:text-slate-400 disabled:no-underline">
              <IconCheck className="size-3.5" strokeWidth={2.6} />{t("bell.mark_read")}
            </button>
          </div>
          {rows.length === 0 ? (
            <div className="px-3 pb-5 pt-3 text-center">
              <IconCheck className="mx-auto size-10 text-leaf" strokeWidth={1.4} />
              <p className="mt-2 text-sm font-medium">{t("bell.empty_title")}</p>
              <p className="text-xs text-muted">{t("bell.empty_body")}</p>
            </div>
          ) : (
            <ul className="max-h-[22rem] overflow-y-auto">
              {rows.slice(0, 12).map((r) => {
                const Icon = r.severity === "critical" ? IconWarning : AREA_ICON[r.area];
                return (
                  <li key={r.key}>
                    <DM.Item asChild>
                      <Link href={r.href} onClick={() => { if (!r.key.startsWith("a-") && r.unread) read.mutate(); }}
                        className={cn("flex gap-3 rounded-xl px-3 py-2.5 outline-none transition-colors data-[highlighted]:bg-slate-50", r.unread && "bg-leaf-soft/40")}>
                        <Icon className={cn("mt-0.5 size-7 shrink-0", r.severity === "critical" ? "text-[#d9707f]" : r.severity === "warning" ? "text-[#d9a441]" : "text-[#5b8fdc]")} strokeWidth={1.5} />
                        <span className="min-w-0 flex-1">
                          <span className="flex items-center gap-2"><span className="truncate text-sm font-semibold">{r.title}</span>{r.unread && <span className={cn("size-2 shrink-0 rounded-full", SEV_DOT[r.severity])} />}</span>
                          {r.body && <span className="mt-0.5 line-clamp-2 block text-xs text-slate-600">{r.body}</span>}
                          <span className="mt-0.5 block text-[11px] text-slate-400">{timeAgo(r.at)}</span>
                        </span>
                      </Link>
                    </DM.Item>
                  </li>
                );
              })}
            </ul>
          )}
          <div className="flex items-center justify-between gap-3 border-t border-line/70 px-3 py-2.5">
            <label className="flex cursor-pointer items-center gap-2 text-xs font-medium text-slate-600">
              <Switch label={t("bell.sound")} size="sm" checked={soundOn} onChange={(v) => { const x = v ? "on" : "off"; writeSoundPref(x); setOwn(x); if (v) playChime(); }} />
              {t("bell.sound")}
            </label>
            <span className="flex gap-3">
              <Link href="/alerts" className="text-xs font-semibold text-leaf-deep hover:underline">{t("bell.all")}</Link>
              <Link href="/settings#notifications" className="text-xs font-semibold text-slate-500 hover:underline">{t("bell.settings")}</Link>
            </span>
          </div>
        </div>
      </DropdownPanel>
    </DropdownRoot>
  );
}
