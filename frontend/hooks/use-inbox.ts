"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { z } from "zod";
import { api, apiVoid } from "@/lib/api/client";

const item = z.object({
  id: z.string(), type: z.string(), severity: z.enum(["info", "warning", "critical"]), title: z.string(), body: z.string().default(""),
  path: z.string().default(""), created_at: z.string(), unread: z.boolean(),
});
export type InboxItem = z.infer<typeof item>;
const feed = z.object({ items: z.array(item), unread: z.number(), sound: z.boolean() });

/** The bell feed: what the platform tells this person, filtered by the organization's display settings. */
export const useInbox = () => useQuery({ queryKey: ["inbox"], retry: false, refetchInterval: 30_000, queryFn: () => api("inbox", feed) });

export function useMarkInboxRead() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: () => apiVoid("inbox/read", { method: "POST", body: "{}" }),
    onSuccess: () => void qc.invalidateQueries({ queryKey: ["inbox"] }),
  });
}

/** Which area an event type belongs to, for grouping and for the icon. */
export function eventArea(type: string): "billing" | "reports" | "automation" | "recommendations" | "connections" | "other" {
  const head = type.split(".")[0];
  if (head === "billing") return "billing";
  if (head === "report") return "reports";
  if (head === "automation") return "automation";
  if (head === "recommendation") return "recommendations";
  if (head === "connection") return "connections";
  return "other";
}
