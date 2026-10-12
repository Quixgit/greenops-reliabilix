"use client";

import { useQuery } from "@tanstack/react-query";
import { z } from "zod";
import { api } from "@/lib/api/client";
import { useCloudQuery } from "@/hooks/use-overview";

const month = z.object({
  month: z.string(), days: z.number(), units: z.number(), cost: z.number(), carbon_g: z.number(),
  cost_per_unit: z.number().nullable(), carbon_g_per_unit: z.number().nullable(),
});
const unitEconomics = z.object({
  currency: z.string(),
  projects: z.array(z.object({
    project_id: z.string(), name: z.string(), unit: z.string(), months: z.array(month),
    cost_change_pct: z.number().nullable(), carbon_change_pct: z.number().nullable(),
  })),
});
export type UnitEconomics = z.infer<typeof unitEconomics>;
export type UnitProject = UnitEconomics["projects"][number];

/** Cost and carbon of one unit of the customer's work, per project and month, over the days units were reported. */
export const useUnitEconomics = (months = 6) => {
  const { cloud, ready } = useCloudQuery();
  const q = useQuery({ queryKey: ["unit-economics", months, cloud], enabled: ready, queryFn: () => api("dashboard/unit-economics", unitEconomics, { months: String(months), cloud }) });
  return ready ? q : { ...q, isLoading: true, isPending: true } as typeof q;
};
