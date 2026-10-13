"use client";

import { useCallback, useEffect, useState, type ReactNode } from "react";
import { cx } from "./ui";

/**
 * The frame every page shares: a title with its one-line purpose and the page's main action on the right, then (optionally) a bar of
 * sections. One page = one job per section, so nothing is a long scroll of unrelated blocks.
 */
export function PageHead({ title, sub, actions, eyebrow }: { title: ReactNode; sub?: ReactNode; actions?: ReactNode; eyebrow?: ReactNode }) {
  return (
    <header className="dxp-head">
      <div style={{ minWidth: 0 }}>
        {eyebrow && <div className="dxp-eyebrow">{eyebrow}</div>}
        <h1>{title}</h1>
        {sub && <p>{sub}</p>}
      </div>
      {actions && <div className="dxp-actions">{actions}</div>}
    </header>
  );
}

export type PageTab = { id: string; label: ReactNode; count?: number | null; hidden?: boolean };

/** The section lives in the address (`#id`), so a link, a reload and the back button land on the same section. */
export function useHashTab(ids: string[], initial: string): [string, (id: string) => void] {
  const [tab, setTab] = useState(initial);
  const key = ids.join("|");
  useEffect(() => {
    const read = () => {
      const h = window.location.hash.slice(1);
      if (h && key.split("|").includes(h)) setTab(h);
    };
    read();
    window.addEventListener("hashchange", read);
    return () => window.removeEventListener("hashchange", read);
  }, [key]);
  const go = useCallback((id: string) => {
    setTab(id);
    window.history.replaceState(null, "", `#${id}`);
  }, []);
  return [ids.includes(tab) ? tab : initial, go];
}

export function PageTabs({ tabs, value, onChange, label }: { tabs: PageTab[]; value: string; onChange: (id: string) => void; label: string }) {
  const shown = tabs.filter((t) => !t.hidden);
  return (
    <div className="dxp-tabs" role="tablist" aria-label={label}>
      {shown.map((t) => (
        <button key={t.id} type="button" role="tab" aria-selected={value === t.id} className={cx(value === t.id && "is-on")} onClick={() => onChange(t.id)}>
          {t.label}
          {t.count != null && <em>{t.count}</em>}
        </button>
      ))}
    </div>
  );
}

/** A tabbed page body: the bar, then the active section (re-mounted so it fades in). */
export function Sections({ tabs, initial, label, children }: { tabs: PageTab[]; initial?: string; label: string; children: (id: string) => ReactNode }) {
  const ids = tabs.filter((t) => !t.hidden).map((t) => t.id);
  const [tab, go] = useHashTab(ids, initial ?? ids[0]);
  return (
    <>
      <PageTabs tabs={tabs} value={tab} onChange={go} label={label} />
      <section key={tab} role="tabpanel" className="dxp-body">{children(tab)}</section>
    </>
  );
}
