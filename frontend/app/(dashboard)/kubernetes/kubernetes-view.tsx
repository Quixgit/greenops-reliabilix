"use client";

import { useState } from "react";
import { ConnectCluster } from "@/components/kubernetes/connect-cluster";
import { IconPlus } from "@/components/ui/icons";
import { ClusterCards, EnergySplit, K8sSetup, K8sStats, RightSizing, WorkloadTable } from "@/components/kubernetes/k8s-board";
import { useK8sClusters } from "@/hooks/use-overview";
import { useT } from "@/i18n/provider";
import { cn } from "@/lib/utils";

export function KubernetesView({ canConnect = false }: { canConnect?: boolean }) {
  const t = useT();
  const [cluster, setCluster] = useState("");
  const [connecting, setConnecting] = useState(false);
  const clusters = useK8sClusters();
  const none = !clusters.isLoading && !clusters.isError && (clusters.data?.length ?? 0) === 0;
  return (
    <div className="page-enter space-y-6">
      <div className="flex flex-wrap items-end justify-between gap-4">
        <div>
          <h1 className="text-2xl font-bold tracking-tight">Kubernetes</h1>
          <p className="mt-1 max-w-3xl text-sm text-muted">{t("k8.subtitle")}</p>
        </div>
        {canConnect && !connecting && (
          <button type="button" onClick={() => setConnecting(true)} className="btn-leaf inline-flex items-center gap-2 rounded-xl px-4 py-2.5 text-sm font-semibold">
            <IconPlus className="size-4" strokeWidth={2.2} />{t("kc.connect")}
          </button>
        )}
      </div>
      <div className={cn("grid transition-[grid-template-rows,opacity] duration-500 ease-out", connecting ? "grid-rows-[1fr] opacity-100" : "grid-rows-[0fr] opacity-0")} aria-hidden={!connecting}>
        <div className="overflow-hidden"><div className="pb-1">{connecting && <ConnectCluster onClose={() => setConnecting(false)} />}</div></div>
      </div>
      {none && !connecting ? <K8sSetup onConnect={canConnect ? () => setConnecting(true) : undefined} /> : none ? null : (
        <>
          <K8sStats />
          <ClusterCards cluster={cluster} onPick={setCluster} />
          <EnergySplit cluster={cluster} />
          <RightSizing />
          <WorkloadTable cluster={cluster} onClear={() => setCluster("")} />
        </>
      )}
    </div>
  );
}
