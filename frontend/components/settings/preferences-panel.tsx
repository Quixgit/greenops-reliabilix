"use client";

import { Card, CardHeader } from "@/components/ui/card";
import { Switch } from "@/components/operator/ui";
import { PERIODS, useUi } from "@/lib/store";
import { usePrefs } from "@/lib/prefs";
import { useT } from "@/i18n/provider";
import { cn } from "@/lib/utils";

function Segmented<T extends string>({ value, onChange, options, label }: { value: T; onChange: (v: T) => void; options: [T, string][]; label: string }) {
  return (
    <div className="inline-flex rounded-xl bg-slate-100 p-0.5 text-sm" role="radiogroup" aria-label={label}>
      {options.map(([k, text]) => (
        <button key={k} type="button" role="radio" aria-checked={value === k} onClick={() => onChange(k)}
          className={cn("rounded-[10px] px-3 py-1.5 font-medium transition", value === k ? "bg-white text-ink shadow-sm" : "text-muted hover:text-ink")}>{text}</button>
      ))}
    </div>
  );
}

function Row({ title, hint, children }: { title: string; hint: string; children: React.ReactNode }) {
  return (
    <li className="flex flex-wrap items-center gap-x-6 gap-y-2 py-3.5">
      <span className="min-w-0 flex-1 basis-60"><span className="block text-sm font-medium">{title}</span><span className="block text-xs text-muted">{hint}</span></span>
      {children}
    </li>
  );
}

/** How the app looks and what it opens with for this person. Stored in this browser only. */
export function PreferencesPanel() {
  const t = useT();
  const { period, setPeriod } = useUi();
  const { density, reduceMotion, carbonUnit, set } = usePrefs();
  return (
    <Card interactive={false}>
      <CardHeader title={t("settings.prefs.title", undefined, "Preferences")} />
      <p className="-mt-2 mb-1 text-xs text-muted">{t("settings.prefs.note", undefined, "Personal display settings. They are kept in this browser and never change what other people in the organization see.")}</p>
      <ul className="divide-y divide-line/70">
        <Row title={t("settings.prefs.period", undefined, "Default period")} hint={t("settings.prefs.period_hint", undefined, "The period every chart and number opens with.")}>
          <Segmented label="Default period" value={period} onChange={setPeriod} options={PERIODS.map((p) => [p.key, p.label.replace("Last ", "")] as [typeof p.key, string])} />
        </Row>
        <Row title={t("settings.prefs.unit", undefined, "Carbon unit")} hint={t("settings.prefs.unit_hint", undefined, "Automatic switches between kg and tonnes by size.")}>
          <Segmented label="Carbon unit" value={carbonUnit} onChange={(v) => set({ carbonUnit: v })} options={[["auto", "Auto"], ["kg", "kg CO₂e"], ["t", "t CO₂e"]]} />
        </Row>
        <Row title={t("settings.prefs.density", undefined, "Density")} hint={t("settings.prefs.density_hint", undefined, "Compact fits more rows and cards on the screen.")}>
          <Segmented label="Density" value={density} onChange={(v) => set({ density: v })} options={[["comfortable", "Comfortable"], ["compact", "Compact"]]} />
        </Row>
        <Row title={t("settings.prefs.motion", undefined, "Reduce motion")} hint={t("settings.prefs.motion_hint", undefined, "Turns off entrance animations, pulses and transitions.")}>
          <Switch label="Reduce motion" checked={reduceMotion} onChange={(v) => set({ reduceMotion: v })} />
        </Row>
      </ul>
    </Card>
  );
}
