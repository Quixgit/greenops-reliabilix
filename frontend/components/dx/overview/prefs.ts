"use client";

import { useStored } from "../ui";

export interface DxPrefs { more: boolean; activity: boolean; heavyOnly: boolean; avg: boolean; spikes: boolean }
export const DEFAULT_PREFS: DxPrefs = { more: false, activity: false, heavyOnly: false, avg: false, spikes: true };

/** What this person chose to see on the page, kept on this device. */
export function useDxPrefs(): [DxPrefs, <K extends keyof DxPrefs>(k: K, v: DxPrefs[K]) => void] {
  const [prefs, setPrefs] = useStored<DxPrefs>("overview", DEFAULT_PREFS);
  const merged = { ...DEFAULT_PREFS, ...prefs };
  return [merged, (k, v) => setPrefs({ ...merged, [k]: v })];
}
