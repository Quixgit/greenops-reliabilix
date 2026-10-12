"use client";

import { useQuery } from "@tanstack/react-query";
import { z } from "zod";
import { api } from "@/lib/api/client";
import { useUi } from "@/lib/store";

const provider = z.object({
  provider: z.string(), status: z.enum(["ok", "not_enough_data"]), data_days: z.number(),
  monthly_committable: z.number(), baseline_daily: z.number(), median_daily: z.number(), peak_daily: z.number(), stable_share_pct: z.number(),
  discount_pct: z.number(), covered_monthly: z.number(), commitment_per_hour: z.number(), monthly_saving: z.number(), annual_saving: z.number(),
  break_even_utilization: z.number(),
  scenarios: z.array(z.object({ utilization_pct: z.number(), monthly_net: z.number() })),
  daily: z.array(z.object({ day: z.string(), cost: z.number() })),
  possibly_committed: z.boolean(),
  // What the cloud already holds and what to do about it (ADR-0054)
  existing_source: z.enum(["", "read", "entered"]).default(""),
  existing_coverage_pct: z.number().nullable().default(null),
  existing_utilization_pct: z.number().nullable().default(null),
  existing_unused_monthly: z.number().nullable().default(null),
  additional_coverage_pct: z.number().default(0),
  advice: z.enum(["buy", "hold", "reduce"]).default("buy"),
});
const commitments = z.object({
  currency: z.string(), from: z.string(), to: z.string(), lookback_days: z.number(), term_years: z.number(), coverage_pct: z.number(),
  providers: z.array(provider), total_monthly_saving: z.number(), total_annual_saving: z.number(), default_discount_pct: z.record(z.string(), z.number()),
});
export type Commitments = z.infer<typeof commitments>;
export type ProviderCommitment = z.infer<typeof provider>;
export type Existing = { coverage?: number; utilization?: number };
export type CommitmentChoice = { lookback: number; term: 1 | 3; coverage: number; discounts: Record<string, number | undefined>; existing?: Record<string, Existing | undefined> };

/** What committing the steady part of compute and database spend would save. A report: nothing is bought. */
export function useCommitments(c: CommitmentChoice) {
  const { projectId } = useUi();
  const q: Record<string, string> = { lookback_days: String(c.lookback), term_years: String(c.term), coverage_pct: String(c.coverage) };
  if (projectId) q.project_id = projectId;
  for (const [p, v] of Object.entries(c.discounts)) if (v !== undefined) q[`discount_${p}`] = String(v);
  for (const [p, e] of Object.entries(c.existing ?? {})) {
    if (e?.coverage !== undefined) q[`existing_coverage_${p}`] = String(e.coverage);
    if (e?.utilization !== undefined) q[`existing_utilization_${p}`] = String(e.utilization);
  }
  return useQuery({ queryKey: ["commitments", q], queryFn: () => api("finops/commitments", commitments, q), placeholderData: (prev) => prev, retry: false });
}
