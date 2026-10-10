import { create } from "zustand";
import { persist } from "zustand/middleware";

export type PeriodKey = "7d" | "30d" | "90d";
export const PERIODS: { key: PeriodKey; label: string; days: number }[] = [
  { key: "7d", label: "Last 7 days", days: 7 },
  { key: "30d", label: "Last 30 days", days: 30 },
  { key: "90d", label: "Last 90 days", days: 90 },
];

type UiState = {
  projectId: string | null; // null = all projects
  period: PeriodKey;
  setProject: (id: string | null) => void;
  setPeriod: (p: PeriodKey) => void;
  focusService: string | null; // a service picked on a chart: its row in Details by Service is highlighted
  setFocusService: (s: string | null) => void;
  navOpen: boolean; // mobile sidebar
  setNavOpen: (o: boolean) => void;
};

/** UI state only (server state lives in TanStack Query). */
export const useUi = create<UiState>()(persist((set) => ({
  projectId: null,
  period: "30d",
  setProject: (projectId) => set({ projectId }),
  setPeriod: (period) => set({ period }),
  focusService: null,
  setFocusService: (focusService) => set({ focusService }),
  navOpen: false,
  setNavOpen: (navOpen) => set({ navOpen }),
}), {
  // The filters follow the user between sections and reloads (a chart that links to a detail page keeps them).
  name: "rlx-filters", skipHydration: true, // restored after mount (providers.tsx): the server render never knows the filters
   partialize: (s) => ({ projectId: s.projectId, period: s.period }),
  storage: { getItem: (k) => { try { const v = localStorage.getItem(k); return v ? JSON.parse(v) : null; } catch { return null; } },
    setItem: (k, v) => { try { localStorage.setItem(k, JSON.stringify(v)); } catch { /* private mode */ } },
    removeItem: (k) => { try { localStorage.removeItem(k); } catch { /* private mode */ } } },
}));

/** Inclusive [from, to] dates (YYYY-MM-DD) for the selected period, ending today (UTC). */
export function periodRange(key: PeriodKey, now = new Date()): { from: string; to: string } {
  const days = PERIODS.find((p) => p.key === key)!.days;
  const to = new Date(Date.UTC(now.getUTCFullYear(), now.getUTCMonth(), now.getUTCDate()));
  const from = new Date(to.getTime() - (days - 1) * 86_400_000);
  return { from: from.toISOString().slice(0, 10), to: to.toISOString().slice(0, 10) };
}
