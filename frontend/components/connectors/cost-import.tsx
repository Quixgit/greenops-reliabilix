"use client";

import { useRef, useState } from "react";
import { Card, CardHeader } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { ConfirmButton } from "@/components/ui/confirm-button";
import { IconDatabase } from "@/components/ui/icons";
import { Skeleton } from "@/components/ui/skeleton";
import { useDeleteImport, useImports, useUploadCost, type CostImport } from "@/hooks/use-imports";
import { useProjects } from "@/hooks/use-overview";
import { useT } from "@/i18n/provider";
import { formatCost } from "@/lib/formatters";
import { shortTime } from "@/lib/format-time";

const VENDORS = ["Datadog", "Snowflake", "MongoDB Atlas", "GitHub", "GitLab", "Vercel", "Cloudflare", "Atlassian", "New Relic", "Other"];
const input = "w-full rounded-xl border border-line bg-white px-3 py-2 text-sm outline-none focus:border-leaf focus:ring-2 focus:ring-leaf/30";

function StatusBadge({ i }: { i: CostImport }) {
  const t = useT();
  const tone = i.status === "done" ? "good" : i.status === "failed" ? "bad" : "neutral";
  return <Badge tone={tone}>{t(`imp.status.${i.status}`)}</Badge>;
}

/** Upload the cost export of a vendor that is not one of the three clouds; what it makes can be taken back file by file. */
export function CostImportPanel({ vendor, onVendor, canWrite }: { vendor: string; onVendor: (v: string) => void; canWrite: boolean }) {
  const t = useT();
  const projects = useProjects();
  const imports = useImports();
  const upload = useUploadCost();
  const remove = useDeleteImport();
  const file = useRef<HTMLInputElement>(null);
  const [project, setProject] = useState("");
  const [currency, setCurrency] = useState("");
  const [chosen, setChosen] = useState<File | null>(null);
  const plist = projects.data ?? [];
  const projectId = project || plist[0]?.id || "";
  const name = (id: string) => plist.find((p) => p.id === id)?.name ?? "—";
  const res = upload.data;
  return (
    <section id="cost-import" className="space-y-4">
      <Card index={0} interactive={false}>
        <CardHeader icon={<IconDatabase className="size-[18px] text-sky-500" />} title={t("imp.title")} info={t("imp.info")} />
        <p className="max-w-3xl text-sm text-muted">{t("imp.lead")}</p>
        {canWrite ? (
          <form className="mt-4 grid gap-3 sm:grid-cols-2 lg:grid-cols-[1fr_1fr_8rem_1.4fr_auto]" onSubmit={(e) => { e.preventDefault(); if (chosen && projectId && vendor.trim()) upload.mutate({ file: chosen, projectId, vendor: vendor.trim(), currency: currency.trim().toUpperCase() }); }}>
            <label className="space-y-1 text-xs font-medium text-slate-600">{t("imp.vendor")}
              <input list="imp-vendors" value={vendor} onChange={(e) => onVendor(e.target.value)} className={input} placeholder="Datadog" maxLength={60} required />
              <datalist id="imp-vendors">{VENDORS.map((v) => <option key={v} value={v} />)}</datalist>
            </label>
            <label className="space-y-1 text-xs font-medium text-slate-600">{t("imp.project")}
              <select value={projectId} onChange={(e) => setProject(e.target.value)} className={input}>{plist.map((p) => <option key={p.id} value={p.id}>{p.name}</option>)}</select>
            </label>
            <label className="space-y-1 text-xs font-medium text-slate-600">{t("imp.currency")}
              <input value={currency} onChange={(e) => setCurrency(e.target.value)} className={input} placeholder="USD" maxLength={3} />
            </label>
            <label className="space-y-1 text-xs font-medium text-slate-600">{t("imp.file")}
              <input ref={file} type="file" accept=".csv,text/csv,text/plain" onChange={(e) => setChosen(e.target.files?.[0] ?? null)} className={`${input} file:mr-3 file:rounded-lg file:border-0 file:bg-leaf-soft file:px-3 file:py-1 file:text-xs file:font-semibold file:text-leaf-deep`} required />
            </label>
            <button disabled={upload.isPending || !chosen || !projectId || !vendor.trim()} className="self-end rounded-xl btn-leaf px-4 py-2.5 text-sm font-semibold text-white active:scale-[0.98] disabled:opacity-60">{upload.isPending ? t("imp.uploading") : t("imp.upload")}</button>
          </form>
        ) : <p className="mt-3 text-sm text-muted">{t("imp.readonly")}</p>}
        {upload.isError && <p role="alert" className="mt-3 rounded-xl bg-[#f8eceb] px-3 py-2 text-sm text-[#8a403d]">{upload.error.message}</p>}
        {res && !upload.isError && (
          <div className="mt-3 rounded-xl bg-leaf-soft/60 px-3 py-2 text-sm text-slate-700">
            <p className="font-medium">{t("imp.accepted", { rows: res.preview.rows_read, services: res.preview.services, total: formatCost(res.preview.total_cost, res.preview.currency), from: res.preview.from ?? "", to: res.preview.to ?? "" })}</p>
            <p className="mt-1 text-xs text-muted">{t("imp.columns")} {Object.entries(res.preview.columns).map(([k, v]) => `${k}: ${v}`).join(" · ")}</p>
            {(res.preview.warnings ?? []).map((w) => <p key={w} className="mt-1 text-xs text-[#77621a]">⚠ {w}</p>)}
          </div>
        )}
        <details className="mt-3 text-xs text-muted">
          <summary className="cursor-pointer font-medium text-slate-600">{t("imp.format")}</summary>
          <p className="mt-2 max-w-3xl">{t("imp.format_body")}</p>
          <pre className="mt-2 overflow-x-auto rounded-lg bg-slate-50 p-3 text-[11px] leading-snug text-slate-700">{"ChargePeriodStart,ServiceName,BilledCost,BillingCurrency,Tags\n2026-09-01,Log Management,42.10,USD,\"{\"\"team\"\":\"\"platform\"\"}\"\n2026-09-01,APM,65.00,USD,"}</pre>
        </details>
      </Card>
      {imports.isLoading ? <Skeleton className="h-24" /> : (imports.data?.items.length ?? 0) > 0 && (
        <Card index={1} interactive={false} className="space-y-1">
          {imports.data!.items.map((i) => (
            <div key={i.id} className="grid items-center gap-x-4 gap-y-1 rounded-xl px-2 py-2.5 hover:bg-slate-50 sm:grid-cols-[minmax(0,1fr)_9rem_8rem_auto]">
              <span className="min-w-0">
                <span className="block truncate text-sm font-medium">{i.vendor} <span className="font-normal text-muted">· {i.filename || t("imp.nofile")}</span></span>
                <span className="block truncate text-[11px] text-muted">{name(i.project_id)} · {shortTime(i.created_at)}{i.from ? ` · ${i.from} → ${i.to}` : ""}{i.status === "done" ? ` · ${t("imp.rows", { rows: i.rows_imported, services: i.services })}` : ""}</span>
                {i.error && <span className="block truncate text-[11px] text-[#8a403d]" title={i.error}>{i.error}</span>}
                {i.warnings.map((w) => <span key={w} className="block truncate text-[11px] text-[#77621a]" title={w}>⚠ {w}</span>)}
              </span>
              <span className="text-sm font-semibold tabular-nums sm:text-right">{i.status === "done" ? formatCost(i.total_cost, i.currency) : ""}</span>
              <span><StatusBadge i={i} /></span>
              {canWrite && (i.status === "done" || i.status === "failed") ? (
                <ConfirmButton label={t("imp.remove")} confirmLabel={t("imp.remove_confirm")} onConfirm={() => remove.mutate(i.id)} />
              ) : <span />}
            </div>
          ))}
        </Card>
      )}
    </section>
  );
}
