"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { z } from "zod";
import { api, apiVoid } from "@/lib/api/client";

export const costImport = z.object({
  id: z.string(), project_id: z.string(), vendor: z.string(), filename: z.string().default(""), currency: z.string(), status: z.enum(["processing", "done", "failed", "deleting"]),
  error: z.string().default(""), rows_read: z.number(), rows_imported: z.number(), rows_skipped: z.number(), services: z.number(), from: z.string().nullable(), to: z.string().nullable(),
  total_cost: z.number(), warnings: z.array(z.string()).default([]), created_at: z.string(),
});
export type CostImport = z.infer<typeof costImport>;
const list = z.object({ items: z.array(costImport) });
const uploaded = z.object({ import: costImport, preview: z.object({ columns: z.record(z.string(), z.string()).default({}), warnings: z.array(z.string()).nullish(), rows_read: z.number(), rows_skipped: z.number(), services: z.number(), from: z.string().nullable(), to: z.string().nullable(), total_cost: z.number(), currency: z.string() }) });
export type UploadResult = z.infer<typeof uploaded>;

/** The uploaded cost files; while one is being read or removed the list refreshes by itself. */
export const useImports = () => useQuery({
  queryKey: ["imports"],
  queryFn: () => api("imports", list),
  refetchInterval: (q) => (q.state.data?.items.some((i) => i.status === "processing" || i.status === "deleting") ? 2500 : false),
});

export type UploadInput = { file: File; projectId: string; vendor: string; currency: string };

/** The file is the request body (the API takes CSV as it is); a refusal comes back with the reason, which the form shows. */
export function useUploadCost() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (v: UploadInput): Promise<UploadResult> => {
      const qs = new URLSearchParams({ project_id: v.projectId, vendor: v.vendor, filename: v.file.name });
      if (v.currency) qs.set("currency", v.currency);
      const res = await fetch(`/api/proxy/imports/cost?${qs}`, { method: "POST", headers: { "Content-Type": "text/csv" }, body: v.file });
      const body = (await res.json().catch(() => ({}))) as { detail?: string; title?: string };
      if (!res.ok) throw new Error(body.detail || body.title || `HTTP ${res.status}`);
      return uploaded.parse(body);
    },
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ["imports"] });
      for (const k of ["finops-summary", "tag-keys", "showback", "trend", "services"]) void qc.invalidateQueries({ queryKey: [k] });
    },
  });
}

export function useDeleteImport() {
  const qc = useQueryClient();
  return useMutation({ mutationFn: (id: string) => apiVoid(`imports/${id}`, { method: "DELETE" }), onSuccess: () => void qc.invalidateQueries({ queryKey: ["imports"] }) });
}
