"use client";

import { SOURCE_LABEL } from "@/components/charts/intensity-chart";
import { Badge } from "@/components/ui/badge";
import { Card, CardHeader } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { useCarbonSummary, useIntensityHistory, useMethodologyVersions } from "@/hooks/use-overview";
import { useT } from "@/i18n/provider";

const SOURCES: { id: string; key: string }[] = [
  { id: "electricitymaps", key: "method.src.electricitymaps" }, { id: "uk_carbon_intensity", key: "method.src.uk" },
  { id: "eia", key: "method.src.eia" }, { id: "energy_charts", key: "method.src.energy_charts" }, { id: "ieso", key: "method.src.ieso" }, { id: "hydroquebec", key: "method.src.hydroquebec" }, { id: "entsoe", key: "method.src.entsoe" }, { id: "ons", key: "method.src.ons" }, { id: "ember", key: "method.src.ember" },
];

/** What the figures rest on: sources, formulas, coefficients, assumptions, accuracy and limits, in one place. */
export function MethodologyView() {
  const t = useT();
  const sum = useCarbonSummary();
  const versions = useMethodologyVersions();
  const priority = useIntensityHistory(undefined).data?.policy.priority ?? [];
  const m = sum.data?.methodology;
  const ordered = [...SOURCES].sort((a, b) => (priority.indexOf(a.id) + 99) % 99 - (priority.indexOf(b.id) + 99) % 99);
  return (
    <div className="mx-auto max-w-[980px] space-y-5">
      <div className="rise">
        <h1 className="text-2xl font-bold tracking-tight text-ink">{t("method.title")}</h1>
        <p className="mt-1 text-[15px] text-slate-600">{t("method.subtitle")}</p>
      </div>

      <Card index={1} interactive={false}>
        <CardHeader info={t("info.method_current")} title={t("method.current")} action={m && <Badge tone={m.status === "verified" ? "good" : "warn"}>{m.status}</Badge>} />
        {sum.isLoading ? <Skeleton className="h-20" /> : m ? (
          <div className="space-y-2 text-sm">
            <p><b>{m.version}</b> — {m.provenance}</p>
            <ul className="list-disc space-y-1 pl-5 text-slate-700">{m.caveats.map((c) => <li key={c}>{c}</li>)}</ul>
            <p className="text-xs text-muted">{t("method.immutable")}</p>
          </div>
        ) : <p className="text-sm text-muted">{t("method.none")}</p>}
        {(versions.data?.length ?? 0) > 1 && (
          <ul className="mt-3 divide-y divide-line/70 rounded-xl border border-line text-sm">
            {versions.data!.map((v) => (
              <li key={v.version} className="flex items-center justify-between gap-3 px-3 py-2">
                <span><b>{v.version}</b> <span className="text-muted">· {v.provenance}</span></span>
                <Badge tone={v.active ? "good" : "neutral"}>{v.active ? t("method.active") : v.status}</Badge>
              </li>
            ))}
          </ul>
        )}
      </Card>

      <Card index={2} interactive={false}>
        <CardHeader info={t("info.method_formulas")} title={t("method.formulas")} />
        <div className="space-y-3 text-sm leading-relaxed text-slate-700">
          <p><code className="rounded bg-slate-100 px-1.5 py-0.5">{t("method.f.energy")}</code></p>
          <p><code className="rounded bg-slate-100 px-1.5 py-0.5">{t("method.f.carbon")}</code></p>
          <p><code className="rounded bg-slate-100 px-1.5 py-0.5">{t("method.f.sci")}</code></p>
          <p className="text-xs text-muted">{t("method.f.note")}</p>
        </div>
      </Card>

      <Card index={3} interactive={false}>
        <CardHeader info={t("info.method_sources")} title={t("method.sources")} />
        <p className="mb-3 text-sm text-slate-600">{t("method.sources_intro")}</p>
        <ol className="space-y-2.5">
          {ordered.map((s, i) => (
            <li key={s.id} className="flex gap-3 rounded-xl border border-line/70 p-3 text-sm">
              <span className="grid size-6 shrink-0 place-items-center rounded-full bg-slate-100 text-xs font-semibold">{i + 1}</span>
              <span><b>{SOURCE_LABEL[s.id] ?? s.id}</b><span className="block text-slate-600">{t(s.key)}</span></span>
            </li>
          ))}
        </ol>
      </Card>

      <Card index={4} interactive={false}>
        <CardHeader info={t("info.method_limits")} title={t("method.limits")} />
        <ul className="list-disc space-y-1.5 pl-5 text-sm text-slate-700">
          {["method.l.operational", "method.l.estimate", "method.l.history", "method.l.relocation", "method.l.units"].map((k) => <li key={k}>{t(k)}</li>)}
        </ul>
      </Card>
    </div>
  );
}
