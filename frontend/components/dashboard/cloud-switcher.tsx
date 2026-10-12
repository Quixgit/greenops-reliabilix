"use client";

import { useEffect, useLayoutEffect, useRef, useState } from "react";
import { CloudMark } from "@/components/dashboard/cloud-mark";
import { STALE_AFTER, daysBehind } from "@/components/dashboard/cloud-costs";
import { useCloudList, useCloudScope } from "@/hooks/use-overview";
import { useT } from "@/i18n/provider";
import { cloudMeta } from "@/lib/clouds";
import { formatCost, formatNative } from "@/lib/formatters";
import { useUi } from "@/lib/store";
import { cn } from "@/lib/utils";

/**
 * Which cloud the page is about. Clouds are separate providers with their own currency, metrics and emissions, so there is no
 * "all clouds" total: one slim pill with a thumb that glides to the chosen cloud. The amount and the currency are in the tooltip, not on
 * the control, so it stays thin however many clouds there are.
 */
export function CloudSwitcher() {
  const t = useT();
  const list = useCloudList();
  const scope = useCloudScope();
  const setCloud = useUi((s) => s.setCloudFilter);
  const clouds = list.data ?? [];
  const row = useRef<HTMLDivElement>(null);
  const [thumb, setThumb] = useState<{ x: number; w: number } | null>(null);
  const [glide, setGlide] = useState(false); // no animation for the first placement

  // a remembered choice that no longer has data is dropped (the first cloud is shown instead)
  useEffect(() => { if (list.data && scope === null) setCloud(null); }, [list.data, scope, setCloud]);

  const place = () => {
    const el = row.current?.querySelector<HTMLElement>('[aria-checked="true"]');
    if (el) setThumb({ x: el.offsetLeft, w: el.offsetWidth });
  };
  useLayoutEffect(() => { place(); }, [scope, clouds.length]); // eslint-disable-line react-hooks/exhaustive-deps
  useEffect(() => {
    const id = requestAnimationFrame(() => setGlide(true));
    const ro = typeof ResizeObserver !== "undefined" && row.current ? new ResizeObserver(place) : null;
    if (ro && row.current) ro.observe(row.current);
    return () => { cancelAnimationFrame(id); ro?.disconnect(); };
  }, [clouds.length]); // eslint-disable-line react-hooks/exhaustive-deps

  if (clouds.length < 2) return null;
  const move = (d: number) => {
    const i = Math.max(0, clouds.findIndex((c) => c.provider === scope));
    setCloud(clouds[(i + d + clouds.length) % clouds.length].provider);
  };
  return (
    <div ref={row} role="radiogroup" aria-label={t("ov3.switch.label")}
      onKeyDown={(e) => { if (e.key === "ArrowRight" || e.key === "ArrowDown") { e.preventDefault(); move(1); } else if (e.key === "ArrowLeft" || e.key === "ArrowUp") { e.preventDefault(); move(-1); } }}
      className="relative inline-flex max-w-full flex-wrap items-center rounded-full bg-slate-100/80 p-0.5 ring-1 ring-line/60">
      {thumb && (
        <span aria-hidden className={cn("absolute inset-y-0.5 left-0 rounded-full bg-white shadow-[0_1px_3px_rgba(31,52,110,.14)] ring-1 ring-black/5", glide && "transition-[transform,width] duration-300 ease-[var(--ease-out-soft)]")}
          style={{ width: thumb.w, transform: `translateX(${thumb.x}px)` }} />
      )}
      {clouds.map((c) => {
        const m = cloudMeta(c.provider), behind = daysBehind(c.last_day), gap = behind > STALE_AFTER, active = scope === c.provider;
        const own = c.native.length === 1 ? c.native[0] : null;
        const day = c.last_day ? new Date(c.last_day).toLocaleDateString(undefined, { month: "short", day: "numeric", timeZone: "UTC" }) : "—";
        const money = own ? formatNative(own.amount, own.currency) : formatCost(c.cost);
        return (
          <button key={c.provider} type="button" role="radio" aria-checked={active} tabIndex={active ? 0 : -1} onClick={() => setCloud(c.provider)}
            title={`${m.name} · ${money} · ${gap ? t("ov3.clouds.stale", { day, days: behind }) : t("ov3.clouds.fresh", { day })}`}
            className="group relative z-10 flex items-center gap-1.5 rounded-full py-1 pl-1.5 pr-3 text-[13px] font-semibold outline-none transition-colors duration-200 focus-visible:ring-2 focus-visible:ring-primary/40">
            <CloudMark provider={c.provider} className={cn("size-[18px] shrink-0 transition-all duration-300", active ? "scale-100" : "scale-90 opacity-70 group-hover:scale-100 group-hover:opacity-100")} />
            <span className={cn("transition-colors duration-200", active ? "text-ink" : "text-slate-500 group-hover:text-ink")}>{m.name}</span>
            {gap && <span className="size-1.5 shrink-0 rounded-full bg-amber-500" aria-label="stale" />}
          </button>
        );
      })}
    </div>
  );
}
