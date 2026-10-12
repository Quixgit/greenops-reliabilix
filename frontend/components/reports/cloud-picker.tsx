"use client";

import { CloudMark } from "@/components/dashboard/cloud-mark";
import { CheckChip, Switch } from "@/components/ui/controls";
import { useT } from "@/i18n/provider";
import { cloudMeta } from "@/lib/clouds";

export interface PickableCloud { provider: string; native: { currency: string; amount: number }[] }

/**
 * Which clouds of the chosen project a report covers. `value` is null for "all clouds" (the default) or the list that is ticked; the two are
 * kept apart on purpose, so a person can untick down to a single cloud (Azure only) and the list never falls back to "all" on its own.
 * It is shown only when the project has more than one cloud; each is marked with its billing currency, because the file keeps every cloud
 * in its own currency.
 */
export function CloudPicker({ clouds, value, onChange }: { clouds: PickableCloud[]; value: string[] | null; onChange: (v: string[] | null) => void }) {
  const t = useT();
  if (clouds.length < 2) return null;
  const all = value === null;
  const ticked = value ?? clouds.map((c) => c.provider);
  const toggle = (id: string, on: boolean) => {
    const next = on ? [...new Set([...ticked, id])] : ticked.filter((x) => x !== id);
    if (next.length > 0) onChange(next); // at least one cloud stays: an empty report is not a choice
  };
  return (
    <div>
      <div className="mb-1.5 flex flex-wrap items-center justify-between gap-2">
        <p className="text-xs font-semibold uppercase tracking-wide text-slate-500">{t("rp.step.clouds")}</p>
        <Switch checked={all} onChange={(v) => onChange(v ? null : clouds.map((c) => c.provider))} label={t("rp.clouds.all")} />
      </div>
      <div className="flex flex-wrap gap-1.5">
        {clouds.map((c) => {
          const m = cloudMeta(c.provider);
          const on = ticked.includes(c.provider);
          return (
            <span key={c.provider} className={all ? "pointer-events-none opacity-70" : undefined}>
              <CheckChip checked={on} onChange={(v) => toggle(c.provider, v)} color={m.color}>
                <CloudMark provider={c.provider} className="size-4" />{m.name}
                <span className="rounded bg-slate-100 px-1 text-[10px] font-bold text-slate-500">{c.native.map((n) => n.currency).join("+")}</span>
              </CheckChip>
            </span>
          );
        })}
      </div>
      <p className="mt-1.5 text-xs text-muted">{all ? t("rp.clouds.hint_all") : t("rp.clouds.hint_some", { count: ticked.length })}</p>
    </div>
  );
}
