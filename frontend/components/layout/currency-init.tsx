"use client";

import { setDefaultCurrency } from "@/lib/formatters";

/**
 * Tells the formatters which currency this organization's money is in, before the page below renders. It runs in the browser
 * only: the formatter's default is a module variable, and on the server that variable would be shared by every request.
 */
export function CurrencyInit({ currency }: { currency: string }) {
  if (typeof window !== "undefined") setDefaultCurrency(currency);
  return null;
}
