"use client";

import { GateHistory, GatePlayground, GatePolicies, GateStats, Integrate } from "@/components/cicd/cicd-board";
import { useT } from "@/i18n/provider";

export function CiCdView({ canEdit }: { canEdit: boolean }) {
  const t = useT();
  return (
    <div className="page-enter space-y-6">
      <div>
        <h1 className="text-2xl font-bold tracking-tight">CI/CD</h1>
        <p className="mt-1 max-w-3xl text-sm text-muted">{t("cd.subtitle")}</p>
      </div>
      <GateStats />
      <div className="grid gap-5 xl:grid-cols-[minmax(0,1.5fr)_minmax(0,1fr)]">
        <GatePlayground />
        <GatePolicies canEdit={canEdit} />
      </div>
      <Integrate canKeys={canEdit} />
      <GateHistory />
    </div>
  );
}
