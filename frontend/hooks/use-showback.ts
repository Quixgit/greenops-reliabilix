"use client";

import { useQuery } from "@tanstack/react-query";
import { z } from "zod";
import { api } from "@/lib/api/client";
import { periodRange, useUi } from "@/lib/store";
import { useCloudQuery } from "@/hooks/use-overview";

const tagKeys = z.object({ currency: z.string(), items: z.array(z.object({ key: z.string(), values: z.number(), coverage_pct: z.number() })) });
const showback = z.object({
  tag: z.string(), currency: z.string(), has_data: z.boolean(), total_cost: z.number(), tagged_cost: z.number(), untagged_cost: z.number(), coverage_pct: z.number(), tags_from: z.string().optional(),
  lines: z.array(z.object({ value: z.string(), cost: z.number(), share_pct: z.number(), top_services: z.array(z.object({ service: z.string(), cost: z.number() })) })),
});
export type Showback = z.infer<typeof showback>;
export type TagKey = z.infer<typeof tagKeys>["items"][number];

function useFilters() {
  const { projectId, period, custom } = useUi();
  void custom;
  const { from, to } = periodRange(period);
  const { cloud } = useCloudQuery();
  return { from, to, project_id: projectId ?? undefined, cloud };
}

/** The tag keys with cost in the selected period, and how much of the bill each explains. */
export const useTagKeys = () => {
  const f = useFilters();
  return useQuery({ queryKey: ["tag-keys", f], queryFn: () => api("finops/tag-keys", tagKeys, f) });
};

/** The bill of the selected period split by the values of one tag key. */
export const useShowback = (tag: string | null) => {
  const f = useFilters();
  return useQuery({ queryKey: ["showback", tag, f], enabled: !!tag, queryFn: () => api("finops/showback", showback, { ...f, tag: tag! }) });
};
