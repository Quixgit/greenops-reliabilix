"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { z } from "zod";
import { api, apiVoid } from "@/lib/api/client";
import { report } from "@/lib/api/schemas";

const options = z.object({
  kinds: z.array(z.string()), formats: z.array(z.string()), max_days: z.number(), pdf_rows: z.number(), preview_rows: z.number(),
});
const preview = z.object({
  title: z.string(), period: z.string(), notes: z.array(z.string()).default([]), headers: z.array(z.string()), rows: z.array(z.array(z.string())), total_rows: z.number(),
});
export type ReportPreview = z.infer<typeof preview>;
export type ReportRequest = { kind: string; format: string; project_id: string | null; period_start: string; period_end: string };

/** What the engine offers: kinds, the formats this organization allows, and its limits. */
export const useReportOptions = () => useQuery({ queryKey: ["report-options"], queryFn: () => api("reports/options", options), staleTime: 5 * 60_000 });

/** The head of the table a report would contain, so a person sees what they are about to ask for. */
export function useReportPreview(p: Omit<ReportRequest, "format">, enabled: boolean) {
  return useQuery({
    queryKey: ["report-preview", p], enabled, retry: false, staleTime: 30_000,
    queryFn: () => api("reports/preview", preview, { kind: p.kind, period_start: p.period_start, period_end: p.period_end, project_id: p.project_id ?? undefined }),
  });
}

export function useRequestReport() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (v: ReportRequest) => api("reports", report, undefined, { method: "POST", body: JSON.stringify(v) }),
    onSuccess: () => void qc.invalidateQueries({ queryKey: ["reports"] }),
  });
}

export function useDeleteReport() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => apiVoid(`reports/${id}`, { method: "DELETE" }),
    onSuccess: () => void qc.invalidateQueries({ queryKey: ["reports"] }),
  });
}
