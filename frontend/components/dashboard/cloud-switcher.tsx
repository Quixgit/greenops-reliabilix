"use client";

import { useEffect } from "react";
import { CloudMark } from "@/components/dashboard/cloud-mark";
import { STALE_AFTER, daysBehind } from "@/components/dashboard/cloud-costs";
import { useCloudList, useCloudScope } from "@/hooks/use-overview";
import { useT } from "@/i18n/provider";
import { cloudMeta } from "@/lib/clouds";
import { formatCost, formatNative } from "@/lib/formatters";
import { useUi } from "@/lib/store";
import { cn } from "@/lib/utils";

/**
 * Which cloud the Overview is about. Clouds are separate providers with their own currency, metrics and emissions, so there is no
 * "all clouds" total: each tab carries what that cloud was billed in its own currency, and choosing one shows its figures alone.
 */
export function CloudSwitcher() {
  const t = useT();
  const list = useCloudList();
  const scope = useCloudScope();
  const setCloud = useUi((s) => s.setCloudFilter);
  const clouds = list.data ?? [];
  // a remembered choice that no longer has data is dropped (the first cloud is shown instead)
  useEffect(() => { if (list.data && scope === null) setCloud(null); }, [list.data, scope, setCloud]);
  if (clouds.length < 2) return null;
  const compact = clouds.length > 4; // many clouds: icon and name only, the amount stays in the tooltip, so the row never outgrows the page
  return (
    <div role="radiogroup" aria-label={t("ov3.switch.label")} className="flex max-w-full flex-wrap items-stretch gap-1.5 rounded-2xl bg-slate-100/80 p-1 ring-1 ring-line/70">
      {clouds.map((c) => {
        const m = cloudMeta(c.provider), behind = daysBehind(c.last_day), gap = behind > STALE_AFTER, active = scope === c.provider;
        const own = c.native.length === 1 ? c.native[0] : null;
        const day = c.last_day ? new Date(c.last_day).toLocaleDateString(undefined, { month: "short", day: "numeric", timeZone: "UTC" }) : "—";
        return (
          <button key={c.provider} type="button" role="radio" aria-checked={active} onClick={() => setCloud(c.provider)}
            title={`${own ? formatNative(own.amount, own.currency) : formatCost(c.cost)} · ${gap ? t("ov3.clouds.stale", { day, days: behind }) : t("ov3.clouds.fresh", { day })}`}
            className={cn("group relative flex items-center gap-2.5 rounded-xl py-1.5 text-left",
              compact ? "px-2.5" : "min-w-[9.5rem] px-3",
              " outline-none transition-all duration-300 focus-visible:ring-2 focus-visible:ring-primary/40",
              active ? "bg-white shadow-sm ring-1 ring-black/5" : "hover:bg-white/60")}>
            <CloudMark provider={c.provider} className={cn("size-7 shrink-0 transition-transform duration-300", active ? "scale-100" : "scale-90 opacity-80 group-hover:scale-100 group-hover:opacity-100")} />
            <span className="min-w-0 leading-tight">
              <span className={cn("block truncate text-[13px] font-semibold", active ? "text-ink" : "text-slate-600")}>{m.name}</span>
              <span className={cn("block truncate text-[11px] tabular-nums text-muted", compact && "hidden")}>
                {own ? (own.currency === "" ? formatCost(c.cost) : formatNative(own.amount, own.currency)) : c.native.map((n) => formatNative(n.amount, n.currency)).join(" + ")}
              </span>
            </span>
            <span className={cn("ml-auto size-1.5 shrink-0 rounded-full", gap ? "bg-amber-500" : "bg-leaf")} aria-hidden />
            {active && <span className="absolute inset-x-3 -bottom-px h-0.5 rounded-full" style={{ background: m.color }} />}
          </button>
        );
      })}
    </div>
  );
}
