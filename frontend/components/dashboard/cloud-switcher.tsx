"use client";

import { useEffect } from "react";
import { CloudMark } from "@/components/dashboard/cloud-mark";
import { IconCloud } from "@/components/ui/icons";
import { STALE_AFTER, daysBehind } from "@/components/dashboard/cloud-costs";
import { useCloudList, useCloudScope } from "@/hooks/use-overview";
import { useT } from "@/i18n/provider";
import { cloudMeta } from "@/lib/clouds";
import { useUi } from "@/lib/store";
import { cn } from "@/lib/utils";

/**
 * Which cloud the Overview is about. One cloud at a time shows its cost in the currency it is billed in, on its own arithmetic; "all clouds"
 * is the comparison. Clouds billed in different currencies are never poured into one chart by default.
 */
export function CloudSwitcher() {
  const t = useT();
  const list = useCloudList();
  const scope = useCloudScope();
  const setCloud = useUi((s) => s.setCloudFilter);
  const clouds = list.data ?? [];
  // a scope that no longer has data (the cloud was removed) is dropped rather than leaving an empty page
  useEffect(() => { if (scope && list.data && !list.data.some((c) => c.provider === scope)) setCloud(null); }, [scope, list.data, setCloud]);
  if (clouds.length < 2) return null;
  return (
    <div role="radiogroup" aria-label={t("ov3.switch.label")} className="flex max-w-full flex-wrap items-center gap-1 rounded-2xl bg-slate-100/80 p-1 ring-1 ring-line/70">
      <Seg active={scope === null} onClick={() => setCloud(null)} label={t("ov3.switch.all")} icon={<IconCloud className="size-[18px] text-slate-500" />} sub={t("ov3.switch.n", { count: clouds.length })} />
      {clouds.map((c) => {
        const m = cloudMeta(c.provider), behind = daysBehind(c.last_day), gap = behind > STALE_AFTER;
        const code = c.native.length === 1 ? c.native[0].currency : c.native.map((n) => n.currency).join("+");
        return (
          <Seg key={c.provider} active={scope === c.provider} onClick={() => setCloud(c.provider)} label={m.name} sub={code} color={m.color}
            icon={<CloudMark provider={c.provider} className="size-[22px]" />} gap={gap}
            title={gap ? t("ov3.clouds.stale", { day: c.last_day ? new Date(c.last_day).toLocaleDateString(undefined, { month: "short", day: "numeric", timeZone: "UTC" }) : "—", days: behind }) : undefined} />
        );
      })}
    </div>
  );
}

function Seg({ active, onClick, label, sub, icon, color, gap, title }: { active: boolean; onClick: () => void; label: string; sub?: string; icon: React.ReactNode; color?: string; gap?: boolean; title?: string }) {
  return (
    <button type="button" role="radio" aria-checked={active} onClick={onClick} title={title}
      className={cn("group relative flex items-center gap-2 rounded-xl px-3 py-1.5 text-sm font-semibold outline-none transition-all duration-300 focus-visible:ring-2 focus-visible:ring-primary/40",
        active ? "bg-white text-ink shadow-sm ring-1 ring-black/5" : "text-slate-600 hover:bg-white/60 hover:text-ink")}>
      <span className="transition-transform duration-300 group-hover:scale-110">{icon}</span>
      <span>{label}</span>
      {sub && <span className={cn("rounded-md px-1.5 py-px text-[10px] font-bold tracking-wide", active ? "bg-slate-100 text-slate-600" : "bg-white/70 text-slate-500")}>{sub}</span>}
      {gap && <span className="size-1.5 rounded-full bg-amber-500" aria-label="stale" />}
      {active && color && <span className="absolute inset-x-3 -bottom-px h-0.5 rounded-full" style={{ background: color }} />}
    </button>
  );
}
