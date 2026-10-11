"use client";

import * as DM from "@radix-ui/react-dropdown-menu";
import Link from "next/link";
import { usePathname } from "next/navigation";
import { DropdownItem, DropdownPanel, DropdownRoot } from "@/components/ui/dropdown";
import { IconBell, IconSliders } from "@/components/ui/icons";
import { useInbox } from "@/hooks/use-inbox";
import { useAlerts } from "@/hooks/use-overview";
import { useT } from "@/i18n/provider";
import { cn } from "@/lib/utils";

/** A gear in a circle that holds the two secondary places: Notifications and Settings. */
export function GearMenu({ collapsed }: { collapsed: boolean }) {
  const t = useT();
  const path = usePathname();
  const inbox = useInbox();
  const alerts = useAlerts("new");
  const unread = (inbox.data?.unread ?? 0) + (alerts.data?.length ?? 0);
  const active = path.startsWith("/settings") || path.startsWith("/alerts");
  return (
    <DropdownRoot>
      <DM.Trigger aria-label={t("nav.more", undefined, "Notifications and settings")} title={t("nav.more", undefined, "Notifications and settings")}
        className={cn("group relative flex items-center gap-3 rounded-full outline-none transition-colors", collapsed ? "mx-auto size-11 justify-center" : "mx-3 mb-2 w-[calc(100%-1.5rem)] py-1.5 pl-1.5 pr-4",
          active ? "bg-navy-active text-white" : "text-slate-300 hover:bg-white/10 hover:text-white")}>
        <span className="relative grid size-9 shrink-0 place-items-center rounded-full bg-white/10 ring-1 ring-white/15 transition-colors group-hover:bg-white/15 group-data-[state=open]:bg-leaf/30">
          <IconSliders className="size-5 transition-transform duration-700 group-hover:rotate-90 group-data-[state=open]:rotate-90" strokeWidth={1.6} />
          {unread > 0 && <span className="absolute -right-0.5 -top-0.5 size-3 rounded-full bg-[#e08b98] ring-2 ring-[#12233c]" />}
        </span>
        {!collapsed && <span className="min-w-0 text-left text-[14.5px] font-medium">{t("nav.more_short", undefined, "Settings & alerts")}</span>}
      </DM.Trigger>
      <DropdownPanel align="start">
        <div className="min-w-56">
          <DropdownItem asChild>
            <Link href="/alerts" className="flex items-center gap-3">
              <IconBell className="size-5 text-[#d9a441]" strokeWidth={1.6} />
              <span className="flex-1">{t("nav.alerts", undefined, "Alerts")}</span>
              {unread > 0 && <span className="rounded-full bg-[#fdecef] px-2 py-0.5 text-xs font-bold text-[#c25667]">{unread > 99 ? "99+" : unread}</span>}
            </Link>
          </DropdownItem>
          <DropdownItem asChild>
            <Link href="/settings" className="flex items-center gap-3"><IconSliders className="size-5 text-[#5b8fdc]" strokeWidth={1.6} /><span>{t("nav.settings", undefined, "Settings")}</span></Link>
          </DropdownItem>
        </div>
      </DropdownPanel>
    </DropdownRoot>
  );
}
