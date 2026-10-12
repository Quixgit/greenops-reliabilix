// The language of the interface decides how numbers, money and dates are written (set once by the i18n provider).
let formatLocale = "en-US";
const INTL_TAG: Record<string, string> = { en: "en-US", ru: "ru-RU" };
export function setFormatLocale(locale: string): void {
  formatLocale = INTL_TAG[locale] ?? locale;
}
export const getFormatLocale = () => formatLocale;

/** The unit the person prefers for carbon: "auto" picks kg or t by size. Set once from the preferences. */
let carbonUnit: "auto" | "kg" | "t" = "auto";
export function setCarbonUnit(u: "auto" | "kg" | "t"): void {
  carbonUnit = u;
}
const asTonnes = (kg: number) => carbonUnit === "t" || (carbonUnit === "auto" && Math.abs(kg) >= 1000);

export const nf = (max = 1) => new Intl.NumberFormat(formatLocale, { maximumFractionDigits: max });

/** kg CO2e -> "42.6 t" above 1000 kg, else "320 kg". */
export function formatCarbon(kg: number): string {
  return asTonnes(kg) ? `${nf(carbonUnit === "t" ? 2 : 1).format(kg / 1000)} t` : `${nf(0).format(kg)} kg`;
}

/** The currency cost is shown in when a call does not name one: the organization's reporting currency, set from the summary. */
let defaultCurrency = "USD";
export function setDefaultCurrency(c: string): void {
  if (/^[A-Z]{3}$/.test(c)) defaultCurrency = c;
}
export const getDefaultCurrency = () => defaultCurrency;

export function formatCost(v: number, currency = defaultCurrency): string {
  return new Intl.NumberFormat(formatLocale, { style: "currency", currency, maximumFractionDigits: 0 }).format(v);
}

/** Cost with cents below 100 (a namespace that costs 0.94 must not read as 0), whole units above. */
export function formatCostFine(v: number, currency = defaultCurrency): string {
  const d = Math.abs(v) < 100 ? 2 : 0;
  return new Intl.NumberFormat(formatLocale, { style: "currency", currency, minimumFractionDigits: d, maximumFractionDigits: d }).format(v);
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

/** The number and unit of formatCarbon separately, so the number can be animated while the unit stays put. */
export function carbonParts(kg: number): { n: number; unit: "t" | "kg" } {
  return asTonnes(kg) ? { n: kg / 1000, unit: "t" } : { n: kg, unit: "kg" };
}

/** Change between the first and the second half of a series (null when it cannot be computed honestly). */
export function halfTrend(values: number[]): number | null {
  if (values.length < 4) return null;
  const mid = Math.floor(values.length / 2);
  const avg = (a: number[]) => a.reduce((x, y) => x + y, 0) / a.length;
  return pctChange(avg(values.slice(mid)), avg(values.slice(0, mid)));
}
