"use client";

import { useQueries } from "@tanstack/react-query";
import { IconFlow as Workflow, IconShield as ShieldCheck } from "@/components/ui/icons";
import { z } from "zod";
import { Skeleton } from "@/components/ui/skeleton";
import { useProjects } from "@/hooks/use-overview";
import { useT } from "@/i18n/provider";
import { api } from "@/lib/api/client";
import { FootLink, Panel, Row, Status } from "./organization-tab";

const policy = z.object({
  allowed_regions: z.array(z.string()), ci_carbon_block_kg: z.number().nullable(), ci_cost_block: z.number().nullable(),
  auto_apply_types: z.array(z.string()).default([]),
});

/** What may happen on its own, project by project; the rules themselves are edited in the project policy. */
export function AutomationTab({ go }: { go: (section: string) => void }) {
  const t = useT();
  const projects = useProjects();
  const policies = useQueries({
    queries: (projects.data ?? []).map((p) => ({ queryKey: ["policy", p.id], queryFn: () => api(`projects/${p.id}/policy`, policy) })),
  });
  return (
    <div className="grid gap-4 lg:grid-cols-2">
      <Panel info={t("info.set_auto")} index={0} icon={Workflow} title={t("settings.auto.title")} hint={t("settings.auto.hint")}
        foot={<FootLink href="/automation">{t("settings.auto.open")}</FootLink>}>
        {projects.isLoading ? <Skeleton className="my-3 h-16" /> : (projects.data ?? []).map((p, i) => {
          const pol = policies[i]?.data;
          return (
            <Row key={p.id} icon={Workflow} label={p.name} onClick={() => go("policies")}
              value={!pol ? "…" : <Status ok={pol.auto_apply_types.length === 0}>{pol.auto_apply_types.length > 0 ? pol.auto_apply_types.join(", ").replace(/_/g, " ") : t("settings.auto.approval")}</Status>} />
          );
        })}
      </Panel>
      <Panel info={t("info.set_auto_rules")} index={1} icon={ShieldCheck} title={t("settings.auto.rules")} hint={t("settings.auto.rules_hint")}
        foot={<FootLink onClick={() => go("policies")}>{t("settings.hub.manage_policies")}</FootLink>}>
        <Row icon={ShieldCheck} label={t("settings.auto.region_moves")} value={t("settings.auto.needs_person")} />
        <Row icon={ShieldCheck} label={t("settings.auto.rightsizing")} value={t("settings.auto.needs_person")} />
        <Row icon={ShieldCheck} label={t("settings.auto.plans")} value={t("settings.auto.second_approval")} />
        <p className="border-t border-line/70 py-2.5 text-xs text-muted">{t("settings.hub.automation_note")}</p>
      </Panel>
    </div>
  );
}
