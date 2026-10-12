/** A plan limit or its usage as the customer reads it: tracked spend is money in US dollars, the rest are counts. */
export function planAmount(resource: string, n: number): string {
  if (resource !== "tracked_spend") return String(n);
  return new Intl.NumberFormat("en-US", { style: "currency", currency: "USD", maximumFractionDigits: 0 }).format(n);
}
