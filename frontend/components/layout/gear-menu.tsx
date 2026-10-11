"use client";

import * as DM from "@radix-ui/react-dropdown-menu";
import Link from "next/link";
import { usePathname } from "next/navigation";
import { IconBell, IconCard, IconShield, IconSliders } from "@/components/ui/icons";
import { useInbox } from "@/hooks/use-inbox";
import { useAlerts } from "@/hooks/use-overview";
import { useT } from "@/i18n/provider";
import { cn } from "@/lib/utils";

const item = "flex cursor-pointer items-center gap-3 rounded-xl px-3 py-2.5 text-[14.5px] font-medium text-slate-200 outline-none transition-colors data-[highlighted]:bg-white/10 data-[highlighted]:text-white";

/**
 * A gear in a circle that holds the secondary places: notifications, settings and plan, and for a platform operator the
 * console. The panel is the menu's own dark navy, not a white card, so it reads as part of the menu it opens from.
 */
export function GearMenu({ collapsed, isOperator = false }: { collapsed: boolean; isOperator?: boolean }) {
  const t = useT();
  const path = usePathname();
  const inbox = useInbox();
  const alerts = useAlerts("new");
  const unread = (inbox.data?.unread ?? 0) + (alerts.data?.length ?? 0);
  const active = path.startsWith("/settings") || path.startsWith("/alerts");
  return (
    <DM.Root modal={false}>
      <DM.Trigger aria-label={t("nav.more", undefined, "Notifications and settings")} title={t("nav.more", undefined, "Notifications and settings")}
        className={cn("group relative flex items-center gap-3 rounded-full outline-none transition-colors", collapsed ? "mx-auto size-11 justify-center" : "mx-3 mb-2 w-[calc(100%-1.5rem)] py-1.5 pl-1.5 pr-4",
          active ? "bg-navy-active text-white" : "text-slate-300 hover:bg-white/10 hover:text-white")}>
        <span className="relative grid size-9 shrink-0 place-items-center rounded-full bg-white/10 ring-1 ring-white/15 transition-colors group-hover:bg-white/15 group-data-[state=open]:bg-leaf/30">
          <IconSliders className="size-5 transition-transform duration-700 group-hover:rotate-90 group-data-[state=open]:rotate-90" strokeWidth={1.6} />
          {unread > 0 && <span className="absolute -right-0.5 -top-0.5 size-3 rounded-full bg-[#e08b98] ring-2 ring-[#12233c]" />}
        </span>
        {!collapsed && <span className="min-w-0 text-left text-[14.5px] font-medium">{t("nav.more_short", undefined, "Settings & alerts")}</span>}
      </DM.Trigger>
      <DM.Portal>
        <DM.Content side={collapsed ? "right" : "top"} align="start" sideOffset={10}
          className="z-50 w-64 origin-[var(--radix-dropdown-menu-content-transform-origin)] rounded-2xl bg-gradient-to-b from-[#16304c] to-[#0f2036] p-1.5 text-sm shadow-[0_18px_44px_rgba(8,15,30,0.45)] ring-1 ring-white/10 data-[state=open]:animate-[rise_0.22s_var(--ease-out-soft)]">
          <DM.Item asChild className={item}>
            <Link href="/alerts"><IconBell className="size-5 text-[#f2cf8a]" strokeWidth={1.6} />
              <span className="flex-1">{t("nav.alerts", undefined, "Alerts")}</span>
              {unread > 0 && <span className="rounded-full bg-[#eba7b3] px-2 py-0.5 text-xs font-bold text-[#5a1f2b]">{unread > 99 ? "99+" : unread}</span>}
            </Link>
          </DM.Item>
          <DM.Item asChild className={item}>
            <Link href="/settings"><IconSliders className="size-5 text-[#8db4ee]" strokeWidth={1.6} /><span>{t("nav.settings", undefined, "Settings")}</span></Link>
          </DM.Item>
          <DM.Item asChild className={item}>
            <Link href="/settings#plan"><IconCard className="size-5 text-leaf" strokeWidth={1.6} /><span>{t("nav.gear.billing")}</span></Link>
          </DM.Item>
          {isOperator && (<>
            <DM.Separator className="mx-2 my-1.5 h-px bg-white/10" />
            <p className="px-3 pb-1 pt-1.5 text-[11px] font-semibold uppercase tracking-wider text-slate-500">{t("nav.gear.platform")}</p>
            <DM.Item asChild className={item}>
              <Link href="/operator"><IconShield className="size-5 text-[#b8a9ee]" strokeWidth={1.6} /><span>{t("nav.operator", undefined, "Operator console")}</span></Link>
            </DM.Item>
          </>)}
        </DM.Content>
      </DM.Portal>
    </DM.Root>
  );
}
