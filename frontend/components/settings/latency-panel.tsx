"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { IconGlobe } from "@/components/ui/icons";
import { Segmented } from "@/components/operator/ui";
import { useT } from "@/i18n/provider";
import { api } from "@/lib/api/client";
import { settingsView } from "@/lib/api/schemas";
import { Panel } from "./organization-tab";

const KEY = "recommendations.max_latency_ms";
const PRESETS = [20, 40, 60, 100, 250];

/** How far a recommended region may be from where the workload runs now. The engine reads the same setting. */
export function LatencyPanel({ canEdit }: { canEdit: boolean }) {
  const t = useT();
  const qc = useQueryClient();
  const settings = useQuery({ queryKey: ["settings", "settings"], queryFn: () => api("settings", settingsView) });
  const item = settings.data?.items.find((i) => i.key === KEY);
  const current = typeof item?.value === "number" ? item.value : 60;
  const patch = useMutation({
    mutationFn: (v: number) => api("settings", settingsView, undefined, { method: "PATCH", body: JSON.stringify({ values: { [KEY]: v } }) }),
    onSuccess: (view) => { qc.setQueryData(["settings", "settings"], view); void qc.invalidateQueries({ queryKey: ["recommendations"] }); },
  });
  // A value set outside the presets (through the API) is shown as it is, not forced onto the nearest preset.
  const options = [...new Set([...PRESETS, current])].sort((a, b) => a - b).map((ms) => ({ value: String(ms), label: PRESETS.includes(ms) ? t(`lat.p${ms}`) : `${ms} ms` }));
  return (
    <Panel index={1} icon={IconGlobe} title={t("lat.title")} hint={t("lat.hint")}>
      <div className="space-y-3 border-t border-line/70 pt-4">
        <p className="text-sm text-slate-700">{t("lat.body")}</p>
        {item ? (
          <div className="overflow-x-auto pb-1"><Segmented label={t("lat.title")} value={String(current)} onChange={(v) => canEdit && !patch.isPending && patch.mutate(Number(v))} options={options} /></div>
        ) : null}
        <p className="text-xs text-muted">{t("lat.note")}</p>
        {patch.isError && <p role="alert" className="text-xs text-[#a8504b]">{patch.error.message}</p>}
      </div>
    </Panel>
  );
}
