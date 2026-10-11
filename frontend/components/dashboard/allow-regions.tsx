"use client";

import { useMutation, useQueries, useQuery, useQueryClient } from "@tanstack/react-query";
import { useEffect, useMemo, useState, type ReactNode } from "react";
import { z } from "zod";
import { Drawer } from "@/components/connectors/drawer";
import { IconCheck, IconGlobe } from "@/components/ui/icons";
import { Skeleton } from "@/components/ui/skeleton";
import { useProjects, useRegionFootprint, useRegions } from "@/hooks/use-overview";
import { useT } from "@/i18n/provider";
import { api } from "@/lib/api/client";
import { intensityColor } from "@/lib/intensity-color";
import { nf } from "@/lib/formatters";
import { REGION_GEO, regionName, regionProvider } from "@/lib/regions";
import { cn } from "@/lib/utils";

const policy = z.object({
  project_id: z.string(), allowed_regions: z.array(z.string()),
  ci_carbon_warn_kg: z.number().nullable(), ci_carbon_block_kg: z.number().nullable(), ci_cost_warn: z.number().nullable(), ci_cost_block: z.number().nullable(),
  alert_carbon_monthly_kg: z.number().nullable().optional(), auto_apply_types: z.array(z.string()).default([]),
});

/**
 * The step that makes recommendations appear: say where a project may run. Regions in use are offered first and
 * pre-selected; cleaner regions of the same cloud follow with how much cleaner they are. Saving writes the project's
 * policy (everything else in it is kept) and asks for a fresh analysis.
 */
function Content({ onClose }: { onClose: () => void }) {
  const t = useT();
  const qc = useQueryClient();
  const projects = useProjects();
  const grid = useRegions();
  const fp = useRegionFootprint();
  const [projectId, setProjectId] = useState<string>("");
  useEffect(() => { if (!projectId && projects.data?.length) setProjectId(projects.data[0].id); }, [projects.data, projectId]);
  const pol = useQuery({ queryKey: ["policy", projectId], enabled: !!projectId, queryFn: () => api(`projects/${projectId}/policy`, policy) });
  const [picked, setPicked] = useState<Set<string> | null>(null);

  const used = useMemo(() => new Map((fp.data ?? []).map((f) => [f.region, f])), [fp.data]);
  const clouds = useMemo(() => new Set((fp.data ?? []).map((f) => f.provider)), [fp.data]);
  // candidates: every region of a cloud the organization uses that we have a grid reading for
  const options = useMemo(() => {
    const out: { region: string; g: number | null; used: boolean; cloud: string }[] = [];
    const seen = new Set<string>();
    for (const r of grid.data ?? []) {
      const cloud = regionProvider(r.region).toLowerCase();
      if (clouds.size > 0 && !clouds.has(cloud) && !used.has(r.region)) continue;
      if (!REGION_GEO[r.region] && !used.has(r.region)) continue;
      seen.add(r.region); out.push({ region: r.region, g: r.g_per_kwh, used: used.has(r.region), cloud });
    }
    for (const f of fp.data ?? []) if (!seen.has(f.region) && f.region !== "global") out.push({ region: f.region, g: f.g_per_kwh, used: true, cloud: f.provider });
    return out.sort((a, b) => Number(b.used) - Number(a.used) || (a.g ?? 1e9) - (b.g ?? 1e9));
  }, [grid.data, fp.data, clouds, used]);

  const current = pol.data?.allowed_regions;
  const selected = picked ?? new Set(current && current.length ? current : [...used.keys()].filter((r) => r !== "global"));
  const toggle = (r: string) => { const n = new Set(selected); if (n.has(r)) n.delete(r); else n.add(r); setPicked(n); };
  const save = useMutation({
    mutationFn: async () => {
      const p = pol.data!;
      await api(`projects/${projectId}/policy`, policy, undefined, {
        method: "PUT", body: JSON.stringify({
          allowed_regions: [...selected].sort(), ci_carbon_warn_kg: p.ci_carbon_warn_kg, ci_carbon_block_kg: p.ci_carbon_block_kg,
          ci_cost_warn: p.ci_cost_warn, ci_cost_block: p.ci_cost_block, alert_carbon_monthly_kg: p.alert_carbon_monthly_kg ?? null, auto_apply_types: p.auto_apply_types,
        }),
      });
      await api("recommendations/refresh", z.unknown(), undefined, { method: "POST", body: JSON.stringify({ project_id: projectId }) });
    },
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ["policy"] }); void qc.invalidateQueries({ queryKey: ["policies-all"] });
      for (const k of ["recommendations", "optimization", "region-footprint"]) void qc.invalidateQueries({ queryKey: [k] });
      // the analysis runs in the background: look again shortly
      window.setTimeout(() => { for (const k of ["recommendations", "optimization", "region-footprint"]) void qc.invalidateQueries({ queryKey: [k] }); }, 4000);
      onClose();
    },
  });
  void useQueries; // (kept for symmetry with the policy panels)

  const cleanestUsed = Math.min(...[...used.values()].map((f) => f.g_per_kwh ?? 1e9));
  return (
    <div className="space-y-5">
      <p className="text-sm leading-relaxed text-slate-600">{t("allow.intro")}</p>
      {(projects.data?.length ?? 0) > 1 && (
        <label className="block text-sm"><span className="mb-1 block text-xs font-medium text-slate-600">{t("allow.project")}</span>
          <select value={projectId} onChange={(e) => { setProjectId(e.target.value); setPicked(null); }} className="w-full rounded-lg border border-line bg-white px-3 py-2 text-sm outline-none focus:border-teal-600">
            {projects.data!.map((p) => <option key={p.id} value={p.id}>{p.name}</option>)}
          </select>
        </label>
      )}
      {grid.isLoading || fp.isLoading || pol.isLoading ? <Skeleton className="h-64" /> : (
        <ul className="grid gap-2 sm:grid-cols-2">
          {options.slice(0, 40).map((o, i) => {
            const on = selected.has(o.region);
            const better = o.g !== null && Number.isFinite(cleanestUsed) && cleanestUsed < 1e8 && !o.used && o.g < cleanestUsed ? Math.round((1 - o.g / cleanestUsed) * 100) : null;
            return (
              <li key={o.region} className="rise" style={{ "--i": Math.min(i, 12) } as React.CSSProperties}>
                <button type="button" onClick={() => toggle(o.region)} aria-pressed={on}
                  className={cn("flex w-full items-center gap-3 rounded-xl border px-3 py-2.5 text-left transition-all duration-200",
                    on ? "border-teal-600/60 bg-teal-50/70 shadow-sm" : "border-line bg-white hover:border-slate-300 hover:bg-slate-50")}>
                  <span className={cn("grid size-5 shrink-0 place-items-center rounded-md border transition-colors", on ? "border-teal-700 bg-teal-700 text-white" : "border-slate-300 bg-white")}>
                    {on && <IconCheck className="size-3.5" strokeWidth={2.6} />}
                  </span>
                  <span className="min-w-0 flex-1">
                    <span className="block truncate text-sm font-medium">{regionName(o.region)}</span>
                    <span className="block truncate font-mono text-[11px] text-muted">{o.region}{o.used ? ` · ${t("allow.in_use")}` : ""}</span>
                  </span>
                  <span className="flex shrink-0 flex-col items-end text-xs tabular-nums">
                    <span className="flex items-center gap-1.5"><span className="size-2 rounded-full" style={{ background: o.g === null ? "#e4eaf3" : intensityColor(o.g) }} />{o.g === null ? "—" : `${nf(0).format(o.g)} g`}</span>
                    {better !== null && better > 0 && <span className="font-semibold text-emerald-700">−{better}%</span>}
                  </span>
                </button>
              </li>
            );
          })}
        </ul>
      )}
      <p className="text-xs leading-relaxed text-muted">{t("allow.note")}</p>
      {save.isError && <p role="alert" className="text-sm text-bad">{save.error.message}</p>}
      <div className="sticky bottom-0 -mx-1 flex items-center gap-3 bg-white/95 px-1 py-3 backdrop-blur">
        <button type="button" disabled={save.isPending || selected.size === 0 || !pol.data} onClick={() => save.mutate()}
          className="rounded-xl bg-teal-700 px-5 py-2.5 text-sm font-semibold text-white shadow-sm transition-all hover:bg-teal-800 active:scale-[0.98] disabled:opacity-50">
          {save.isPending ? t("allow.saving") : t("allow.save", { count: selected.size })}
        </button>
        <span className="text-xs text-muted">{t("allow.then")}</span>
      </div>
    </div>
  );
}

/** A button that opens the "where may this project run" panel; `children` is its label. */
export function AllowRegions({ children, className }: { children?: ReactNode; className?: string }) {
  const t = useT();
  const [open, setOpen] = useState(false);
  return (
    <>
      <button type="button" onClick={() => setOpen(true)} className={cn("inline-flex items-center gap-1.5 font-semibold text-primary hover:underline", className)}>
        <IconGlobe className="size-4" />{children ?? t("ov2.opt.set_regions")}
      </button>
      <Drawer open={open} onClose={() => setOpen(false)} title={t("allow.title")} closeLabel={t("common.cancel")}>
        {open && <Content onClose={() => setOpen(false)} />}
      </Drawer>
    </>
  );
}
