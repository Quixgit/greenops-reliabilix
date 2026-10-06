export const nf = (max = 1) => new Intl.NumberFormat("en-US", { maximumFractionDigits: max });

/** kg CO2e -> "42.6 t" above 1000 kg, else "320 kg". */
export function formatCarbon(kg: number): string {
  return Math.abs(kg) >= 1000 ? `${nf(1).format(kg / 1000)} t` : `${nf(0).format(kg)} kg`;
}

export function formatCost(v: number, currency = "USD"): string {
  return new Intl.NumberFormat("en-US", { style: "currency", currency, maximumFractionDigits: 0 }).format(v);
}

/** Percent change from previous to current; null when it cannot be computed honestly. */
export function pctChange(current: number, previous: number): number | null {
  if (!isFinite(current) || !isFinite(previous) || previous === 0) return null;
  return ((current - previous) / previous) * 100;
}

export function formatPct(p: number): string {
  return `${nf(1).format(Math.abs(p))}%`;
}

export function timeAgo(iso: string, now = Date.now()): string {
  const s = Math.max(0, Math.round((now - new Date(iso).getTime()) / 1000));
  if (s < 60) return "just now";
  if (s < 3600) return `${Math.floor(s / 60)}m ago`;
  if (s < 86400) return `${Math.floor(s / 3600)}h ago`;
  return `${Math.floor(s / 86400)}d ago`;
}
