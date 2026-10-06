"use client";

import { zodResolver } from "@hookform/resolvers/zod";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { FileText } from "lucide-react";
import { useForm } from "react-hook-form";
import { z } from "zod";
import { Field, input } from "@/components/forms/project-form";
import { Card, CardHeader } from "@/components/ui/card";
import { EmptyState, ErrorState } from "@/components/ui/empty-state";
import { Skeleton } from "@/components/ui/skeleton";
import { api } from "@/lib/api/client";
import { report } from "@/lib/api/schemas";
import { useReports } from "@/hooks/use-overview";
import { periodRange } from "@/lib/store";

const schema = z.object({
  kind: z.enum(["carbon", "sci", "finops"]),
  format: z.enum(["csv", "json", "pdf"]),
  period_start: z.string().min(1, "Required"),
  period_end: z.string().min(1, "Required"),
}).refine((v) => v.period_start <= v.period_end, { path: ["period_end"], message: "End must not be before start" });
type Values = z.infer<typeof schema>;

function ReportForm() {
  const qc = useQueryClient();
  const { from, to } = periodRange("30d");
  const { register, handleSubmit, formState: { errors } } = useForm<Values>({
    resolver: zodResolver(schema), defaultValues: { kind: "carbon", format: "pdf", period_start: from, period_end: to },
  });
  const m = useMutation({
    mutationFn: (v: Values) => api("reports", report, undefined, { method: "POST", body: JSON.stringify(v) }),
    onSuccess: () => void qc.invalidateQueries({ queryKey: ["reports"] }),
  });
  return (
    <form onSubmit={handleSubmit((v) => m.mutate(v))} className="grid gap-3 sm:grid-cols-5 sm:items-start" noValidate>
      <Field label="Report"><select {...register("kind")} className={input}><option value="carbon">Carbon</option><option value="sci">SCI</option><option value="finops">FinOps</option></select></Field>
      <Field label="Format"><select {...register("format")} className={input}><option value="pdf">PDF</option><option value="csv">CSV</option><option value="json">JSON</option></select></Field>
      <Field label="From" error={errors.period_start?.message}><input type="date" {...register("period_start")} className={input} /></Field>
      <Field label="To" error={errors.period_end?.message}><input type="date" {...register("period_end")} className={input} /></Field>
      <button disabled={m.isPending} className="brand-gradient mt-6 rounded-xl px-4 py-2.5 text-sm font-semibold text-white disabled:opacity-60">{m.isPending ? "Requesting…" : "Generate"}</button>
      {m.isError && <p role="alert" className="text-sm text-bad sm:col-span-5">{m.error.message}</p>}
    </form>
  );
}

export function ReportsView({ canCreate }: { canCreate: boolean }) {
  const q = useReports();
  return (
    <div className="space-y-5">
      <h1 className="text-2xl font-bold tracking-tight">Reports</h1>
      {canCreate && <Card><CardHeader title="New report" /><ReportForm /></Card>}
      <Card>
        <CardHeader title="Your reports" />
        {q.isLoading ? <Skeleton className="h-24" /> : q.isError ? <ErrorState /> : q.data!.length === 0 ?
          <EmptyState icon={FileText} text="No reports yet. Generated reports are listed here and can be downloaded when ready." /> : (
            <ul className="divide-y divide-line/70 text-sm">
              {q.data!.map((r) => (
                <li key={r.id} className="flex flex-wrap items-center justify-between gap-2 py-2.5">
                  <span className="font-medium capitalize">{r.kind} · {r.format.toUpperCase()} <span className="font-normal text-muted">{r.period_start} → {r.period_end}</span></span>
                  {r.status === "ready" ? <a className="font-medium text-primary" href={`/api/proxy/reports/${r.id}/download`} download>Download</a>
                    : r.status === "failed" ? <span className="text-bad">Failed{r.error ? `: ${r.error}` : ""}</span>
                    : <span className="text-muted">Rendering…</span>}
                </li>
              ))}
            </ul>
          )}
      </Card>
    </div>
  );
}
