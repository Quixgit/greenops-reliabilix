"use client";

import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { useEffect, useState, type ReactNode } from "react";
import { setCarbonUnit } from "@/lib/formatters";
import { usePrefs } from "@/lib/prefs";
import { useUi } from "@/lib/store";

/** Puts the personal preferences on the page: attributes the stylesheet reads, and the carbon unit of the formatters. */
function PrefsApplier() {
  const { density, reduceMotion, carbonUnit } = usePrefs();
  useEffect(() => {
    const el = document.documentElement;
    el.dataset.density = density;
    el.dataset.motion = reduceMotion ? "reduced" : "full";
    setCarbonUnit(carbonUnit);
  }, [density, reduceMotion, carbonUnit]);
  return null;
}

export function Providers({ children }: { children: ReactNode }) {
  const [client] = useState(() => new QueryClient({
    defaultOptions: { queries: { staleTime: 60_000, retry: 1, refetchOnWindowFocus: false } },
  }));
  useEffect(() => { void useUi.persist.rehydrate(); void usePrefs.persist.rehydrate(); }, []); // filters saved by the previous visit
  return <QueryClientProvider client={client}><PrefsApplier />{children}</QueryClientProvider>;
}
