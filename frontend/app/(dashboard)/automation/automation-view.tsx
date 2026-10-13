"use client";

import { PageHead } from "@/components/dx/page";
import { useState } from "react";
import { Pipeline, SafetyNote, StageList, type Stage } from "@/components/automation/automation-board";
import { Card } from "@/components/ui/card";
import { EmptyState } from "@/components/ui/empty-state";
import { IconFlow } from "@/components/ui/icons";
import { useT } from "@/i18n/provider";

export function AutomationView({ canSee, canAct }: { canSee: boolean; canAct: boolean }) {
  const t = useT();
  const [stage, setStage] = useState<Stage>("review");
  return (
    <div className="page-enter">
      <PageHead title={t("automation.title")} sub={t("au.subtitle")} />
      {!canSee ? (
        <Card interactive={false}><EmptyState icon={IconFlow} text={t("automation.no_access")} /></Card>
      ) : (
        <div className="dxp-body">
          <SafetyNote />
          <Pipeline stage={stage} onStage={setStage} />
          <StageList stage={stage} canAct={canAct} />
        </div>
      )}
    </div>
  );
}
