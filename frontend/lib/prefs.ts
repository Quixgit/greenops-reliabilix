import { create } from "zustand";
import { persist } from "zustand/middleware";

export type Density = "comfortable" | "compact";
export type CarbonUnitPref = "auto" | "kg" | "t";

type Prefs = {
  density: Density;
  reduceMotion: boolean;
  carbonUnit: CarbonUnitPref;
  set: (p: Partial<Omit<Prefs, "set">>) => void;
};

/** Personal display preferences: kept in this browser, never sent anywhere, applied by <PrefsApplier />. */
export const usePrefs = create<Prefs>()(persist((set) => ({
  density: "comfortable", reduceMotion: false, carbonUnit: "auto",
  set: (p) => set(p),
}), {
  name: "rlx-prefs", skipHydration: true,
  partialize: (s) => ({ density: s.density, reduceMotion: s.reduceMotion, carbonUnit: s.carbonUnit }),
  storage: {
    getItem: (k) => { try { const v = localStorage.getItem(k); return v ? JSON.parse(v) : null; } catch { return null; } },
    setItem: (k, v) => { try { localStorage.setItem(k, JSON.stringify(v)); } catch { /* private mode */ } },
    removeItem: (k) => { try { localStorage.removeItem(k); } catch { /* private mode */ } },
  },
}));
