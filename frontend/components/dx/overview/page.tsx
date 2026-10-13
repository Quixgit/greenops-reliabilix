"use client";

import { Kpis } from "./kpis";
import { useDxPrefs } from "./prefs";
import { Accounts, Activity, NextActions, Setup } from "./rail";
import { RegionsPanel } from "./regions";
import { ScopeBar } from "./scope";
import { ServicesPanel } from "./services";
import { BreakdownPanel, TrendPanel } from "./trend";

/** The Overview in the DX interface: the figures, the trend, where it goes, the services, the regions, and what to do next. */
export function DxOverview() {
  const [prefs, set] = useDxPrefs();
  return (
    <div>
      <ScopeBar prefs={prefs} set={set} />
      <Kpis more={prefs.more} />
      <div className="dx-layout" style={{ marginTop: 16 }}>
        <div style={{ display: "grid", gap: 16, minWidth: 0 }}>
          <div className="dx-split">
            <TrendPanel prefs={prefs} set={set} />
            <BreakdownPanel />
          </div>
          <ServicesPanel prefs={prefs} set={set} />
          <RegionsPanel />
        </div>
        <aside className="dx-rail" aria-label="Next">
          <NextActions />
          <Setup />
          <Accounts />
          {prefs.activity && <Activity />}
        </aside>
      </div>
    </div>
  );
}
