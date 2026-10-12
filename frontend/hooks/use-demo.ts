"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { z } from "zod";
import { api, apiVoid } from "@/lib/api/client";

const status = z.object({ active: z.boolean(), project_id: z.string().nullable() });

/** Whether the organization has the sample project (synthetic cloud spend to look around with). */
export const useDemo = () => useQuery({ queryKey: ["demo"], queryFn: () => api("demo", status) });

/** Everything on screen changes when the sample appears or goes, so every query is refreshed. */
function useRefreshAll() {
  const qc = useQueryClient();
  return () => void qc.invalidateQueries();
}
export function useCreateDemo() {
  const all = useRefreshAll();
  return useMutation({ mutationFn: () => api("demo", status, undefined, { method: "POST", body: "{}" }), onSuccess: all });
}
export function useRemoveDemo() {
  const all = useRefreshAll();
  return useMutation({ mutationFn: () => apiVoid("demo", { method: "DELETE" }), onSuccess: all });
}
