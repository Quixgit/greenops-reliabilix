"use client";

import Link from "next/link";
import { useMemo, useState } from "react";
import { Calendar } from "@/components/ui/calendar";
import { content } from "@/content/schema";
import { useCarbonSummary, useCloudList, useCloudScope, useConnections, useProjects } from "@/hooks/use-overview";
import { useT } from "@/i18n/provider";
import { cloudMeta } from "@/lib/clouds";
import { timeAgo } from "@/lib/formatters";
import { PERIODS, periodLabel, useUi } from "@/lib/store";
import { Ico } from "../icons";
import { Check, Hint, Popover, Segmented, Select, Switch, Tag } from "../ui";
import type { DxPrefs } from "./prefs";

const ALL = "__all__";

/** The page title and everything that scopes it: project, period, cloud, and what to show. */
export function ScopeBar({ prefs, set }: { prefs: DxPrefs; set: <K extends keyof DxPrefs>(k: K, v: DxPrefs[K]) => void }) {
  const t = useT();
  const { projectId, setProject, period, setPeriod, custom, setCustom, setCloudFilter } = useUi();
  const projects = useProjects();
  const conns = useConnections();
  const clouds = useCloudList().data ?? [];
  const scope = useCloudScope();
  const carbon = useCarbonSummary().data;
  const [draft, setDraft] = useState<{ from?: string; to?: string }>({});
  const synced = useMemo(() => (conns.data ?? []).map((c) => c.last_sync_at).filter((x): x is string => !!x).sort().pop(), [conns.data]);
  const project = projects.data?.find((p) => p.id === projectId);
  const provisional = carbon?.has_data && carbon.methodology.status === "provisional";

  return (
    <div className="dx-head dx-rise">
      <div style={{ minWidth: 0 }}>
        <h1>{content.pageTitle}</h1>
        <p>
          <span>{project?.name ?? t("header.all_projects")}</span>{scope && <><i className="dx-sep" /><b style={{ fontWeight: 600, color: "var(--dx-ink)" }}>{cloudMeta(scope).name}</b></>}<i className="dx-sep" />
          <span>{periodLabel(period, custom)}</span>
          <i className="dx-sep" /><span>{synced ? t("ov.head.synced", { when: timeAgo(synced) }) : t("ov.head.no_sync")}</span>
          {provisional && (
            <>
              <i className="dx-sep" />
              <Tag tone="butter">{t("dx.provisional", undefined, "Provisional")}</Tag>
              <Hint title={t("dx.provisional.title", undefined, "Provisional estimates")}>
                {t("dx.provisional.text", { version: carbon!.methodology.version }, "Methodology {version} is provisional. Read it before using these figures outside the company.")}{" "}
                <Link href="/methodology" style={{ color: "var(--dx-sky-ink)", fontWeight: 600 }}>{t("dx.provisional.read", undefined, "Read the methodology")}</Link>
              </Hint>
            </>
          )}
        </p>
      </div>
      <div className="dx-head-tools">
        <Select label={t("header.all_projects")} icon="projects" value={projectId ?? ALL} onChange={(v) => setProject(v === ALL ? null : v)}
          options={[{ value: ALL, label: t("header.all_projects") }, ...(projects.data ?? []).map((p) => ({ value: p.id, label: p.name }))]} />
        <Segmented label={t("header.period")} value={period === "custom" ? ("custom" as const) : period}
          onChange={(v) => { if (v !== "custom") setPeriod(v); }}
          options={[...PERIODS.map((p) => ({ value: p.key as string, label: `${p.days}d` })), { value: "custom", label: t("dx.custom", undefined, "Custom") }] as { value: "7d" | "30d" | "90d" | "custom"; label: string }[]} />
        <Popover align="right" trigger={(open, toggle) => (
          <button type="button" className={`dx-iconbtn ${open || period === "custom" ? "is-on" : ""}`} aria-label={t("header.custom_range")} aria-expanded={open} onClick={() => { toggle(); setDraft(custom ?? {}); }}><Ico name="calendar" size={18} /></button>
        )}>
          {(close) => (
            <div className="dx-pop-body" style={{ minWidth: 300 }}>
              <Calendar mode="range" from={draft.from} to={draft.to} max={new Date().toISOString().slice(0, 10)} onPick={(f, to) => setDraft({ from: f, to })} />
              <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", gap: 8, marginTop: 10, paddingTop: 10, borderTop: "1px solid var(--dx-line)" }}>
                <span style={{ color: "var(--dx-muted)", fontSize: 12 }}>{draft.from ? `${draft.from}${draft.to ? ` → ${draft.to}` : ""}` : t("header.pick_range")}</span>
                <button type="button" className="dx-btn is-primary is-sm" disabled={!draft.from} onClick={() => { setCustom(draft.from!, draft.to ?? draft.from!); close(); }}>{t("header.apply")}</button>
              </div>
            </div>
          )}
        </Popover>
        {clouds.length > 1 && (
          <Segmented label={t("dx.cloud", undefined, "Cloud")} value={scope ?? clouds[0].provider}
            onChange={(v) => setCloudFilter(v)} options={clouds.map((c) => ({ value: c.provider, label: cloudMeta(c.provider).name }))} />
        )}
        <Popover align="right" trigger={(open, toggle) => (
          <button type="button" className={`dx-iconbtn ${open ? "is-on" : ""}`} aria-label={t("dx.display", undefined, "Display")} aria-expanded={open} onClick={toggle}><Ico name="sliders" size={18} /></button>
        )}>
          <div className="dx-pop-title">{t("dx.display", undefined, "Display")}</div>
          <div className="dx-pop-body" style={{ display: "grid", gap: 12, paddingTop: 4, minWidth: 250 }}>
            <Switch checked={prefs.more} onChange={(v) => set("more", v)} label={t("dx.show_more", undefined, "More metrics")} />
            <Switch checked={prefs.activity} onChange={(v) => set("activity", v)} label={t("dx.show_activity", undefined, "Recent activity")} />
            <Check checked={prefs.spikes} onChange={(v) => set("spikes", v)} label={t("dx.mark_spikes", undefined, "Mark cost spikes on the chart")} />
            <Check checked={prefs.avg} onChange={(v) => set("avg", v)} label={t("dx.avg7", undefined, "Smooth with a 7-day average")} />
          </div>
        </Popover>
      </div>
    </div>
  );
}
