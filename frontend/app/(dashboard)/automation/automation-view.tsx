"use client";

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
    <div className="page-enter space-y-6">
      <div>
        <h1 className="text-2xl font-bold tracking-tight">{t("automation.title")}</h1>
        <p className="mt-1 max-w-2xl text-sm text-muted">{t("au.subtitle")}</p>
      </div>
      {!canSee ? (
        <Card interactive={false}><EmptyState icon={IconFlow} text={t("automation.no_access")} /></Card>
      ) : (
        <>
          <SafetyNote />
          <Pipeline stage={stage} onStage={setStage} />
          <StageList stage={stage} canAct={canAct} />
        </>
      )}
    </div>
  );
}
