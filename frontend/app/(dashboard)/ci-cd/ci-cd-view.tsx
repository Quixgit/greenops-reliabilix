"use client";

import { PageHead, Sections } from "@/components/dx/page";
import { GateHistory, GatePlayground, GatePolicies, GateStats, Integrate } from "@/components/cicd/cicd-board";
import { useT } from "@/i18n/provider";

export function CiCdView({ canEdit }: { canEdit: boolean }) {
  const t = useT();
  return (
    <div className="page-enter">
      <PageHead title="CI/CD" sub={t("cd.subtitle")} />
      <Sections label="CI/CD" tabs={[
        { id: "gate", label: t("dx.cicd.gate", undefined, "Gate and policies") },
        { id: "history", label: t("dx.cicd.history", undefined, "History") },
        { id: "integrate", label: t("dx.cicd.integrate", undefined, "Integrate") },
      ]}>
        {(tab) => tab === "gate" ? (
          <>
            <GateStats />
            <div className="dxp-split"><GatePlayground /><GatePolicies canEdit={canEdit} /></div>
          </>
        ) : tab === "history" ? <GateHistory /> : <Integrate canKeys={canEdit} />}
      </Sections>
    </div>
  );
}
