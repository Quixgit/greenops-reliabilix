"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { z } from "zod";
import { api } from "@/lib/api/client";
import { periodRange, useUi } from "@/lib/store";

const evaluation = z.object({
  id: z.string(), project_id: z.string(), verdict: z.enum(["PASS", "WARN", "BLOCK"]), carbon_kg_month: z.number(), cost_delta_month: z.number(),
  reasons: z.array(z.string()).default([]), changes: z.number(), regions: z.array(z.string()).default([]), thresholds_applied: z.boolean(),
  source: z.string(), ref: z.string().optional(), created_at: z.string(),
});
export type GateEvaluation = z.infer<typeof evaluation>;
const history = z.object({
  items: z.array(evaluation),
  summary: z.object({ total: z.number(), pass: z.number(), warn: z.number(), block: z.number(), carbon_blocked_kg_month: z.number(), cost_blocked_month: z.number() }),
});
export const gateResult = z.object({
  verdict: z.enum(["PASS", "WARN", "BLOCK"]), carbon_kg_month: z.number(), cost_delta_month: z.number(), reasons: z.array(z.string()).nullish(),
  thresholds_applied: z.boolean(), methodology_version: z.string(), embodied_carbon_included: z.boolean(), caveats: z.array(z.string()).nullish(),
});
export type GateResult = z.infer<typeof gateResult>;
export type GateChange = { kind: "vcpu_hours" | "memory_gb_hours" | "storage_gb_months"; amount: number; region: string; monthly_cost_delta: number };
export type GateInput = { project_id: string; changes: GateChange[]; ref?: string; thresholds?: Record<string, number> };

/** Recorded gate decisions of the selected period (and project), newest first, with the counts by verdict. */
export function useGateHistory(verdict: string) {
  const { projectId, period, custom } = useUi();
  void custom;
  const { from, to } = periodRange(period);
  return useQuery({
    queryKey: ["ci-evaluations", from, to, projectId, verdict],
    queryFn: () => api("ci/evaluations", history, { from, to, project_id: projectId ?? undefined, verdict: verdict || undefined, limit: "200" }),
  });
}

/** Ask the gate from the console: the same call a pipeline makes, recorded like any other. */
export function useTryGate() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (v: GateInput) => api("ci/evaluate", gateResult, undefined, { method: "POST", body: JSON.stringify({ ...v, ref: v.ref || "console test" }) }),
    onSuccess: () => void qc.invalidateQueries({ queryKey: ["ci-evaluations"] }),
  });
}

/** Pipeline decisions of the last 30 days (the Connectors page shows which CI system is in use). */
export function useGateRuns30d() {
  const to = new Date().toISOString().slice(0, 10);
  const from = new Date(Date.now() - 30 * 86400_000).toISOString().slice(0, 10);
  return useQuery({
    queryKey: ["ci-evaluations-30d", to],
    queryFn: () => api("ci/evaluations", history, { from, to, limit: "500" }),
    staleTime: 60_000,
  });
}
