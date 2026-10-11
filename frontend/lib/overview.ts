// The arithmetic behind the Overview: run-rate, short-term trend and month-end forecast from the daily series the API
// already gives, so every number on the page says what it was computed from and nothing is invented.
export interface DayPoint { day: string; cost: number; carbon_kg_co2e: number }

const sum = (xs: number[]) => xs.reduce((a, b) => a + b, 0);

export interface Rollup {
  days: number;
  cost: number; carbon: number;
  costPerDay: number; carbonPerDay: number;
  /** Last 7 days against the 7 before them (percent); null with fewer than 14 days of data. */
  costTrend7: number | null; carbonTrend7: number | null;
  /** Grams of CO2e per unit of currency spent. */
  gramsPerCurrency: number | null;
}

export function rollup(points: DayPoint[]): Rollup {
  const days = points.length;
  const cost = sum(points.map((p) => p.cost)), carbon = sum(points.map((p) => p.carbon_kg_co2e));
  const change = (key: "cost" | "carbon_kg_co2e") => {
    if (days < 14) return null;
    const last = sum(points.slice(-7).map((p) => p[key])), prev = sum(points.slice(-14, -7).map((p) => p[key]));
    return prev > 0 ? ((last - prev) / prev) * 100 : null;
  };
  return {
    days, cost, carbon,
    costPerDay: days ? cost / days : 0, carbonPerDay: days ? carbon / days : 0,
    costTrend7: change("cost"), carbonTrend7: change("carbon_kg_co2e"),
    gramsPerCurrency: cost > 0 ? (carbon * 1000) / cost : null,
  };
}

/**
 * Cost expected by the end of the month of `now`: what has been spent this month plus the average of the last
 * seven days for the days that remain. null when this month has no data yet (a forecast from nothing is a guess).
 */
export function monthForecast(points: DayPoint[], now: Date = new Date()): { spent: number; forecast: number; daysLeft: number } | null {
  const y = now.getUTCFullYear(), m = now.getUTCMonth();
  const inMonth = points.filter((p) => { const d = new Date(p.day); return d.getUTCFullYear() === y && d.getUTCMonth() === m; });
  if (inMonth.length === 0) return null;
  const dim = new Date(Date.UTC(y, m + 1, 0)).getUTCDate();
  const lastDay = Math.max(...inMonth.map((p) => new Date(p.day).getUTCDate()));
  const daysLeft = Math.max(0, dim - lastDay);
  const recent = points.slice(-7);
  const avg = recent.length ? sum(recent.map((p) => p.cost)) / recent.length : 0;
  const spent = sum(inMonth.map((p) => p.cost));
  return { spent, forecast: spent + avg * daysLeft, daysLeft };
}

/** Moving average over `window` days (the first points average what exists), to calm a noisy daily series. */
export function movingAverage(values: number[], window = 7): number[] {
  return values.map((_, i) => {
    const from = Math.max(0, i - window + 1);
    const slice = values.slice(from, i + 1);
    return sum(slice) / slice.length;
  });
}

/** Share of a total, 0-100, with a floor so a visible bar never disappears. */
export const share = (part: number, total: number) => (total > 0 ? (part / total) * 100 : 0);
