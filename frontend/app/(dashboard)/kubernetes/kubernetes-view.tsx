"use client";

import { useState } from "react";
import { ClusterCards, EnergySplit, K8sSetup, K8sStats, RightSizing, WorkloadTable } from "@/components/kubernetes/k8s-board";
import { useK8sClusters } from "@/hooks/use-overview";
import { useT } from "@/i18n/provider";

export function KubernetesView() {
  const t = useT();
  const [cluster, setCluster] = useState("");
  const clusters = useK8sClusters();
  const none = !clusters.isLoading && !clusters.isError && (clusters.data?.length ?? 0) === 0;
  return (
    <div className="page-enter space-y-6">
      <div>
        <h1 className="text-2xl font-bold tracking-tight">Kubernetes</h1>
        <p className="mt-1 max-w-3xl text-sm text-muted">{t("k8.subtitle")}</p>
      </div>
      {none ? <K8sSetup /> : (
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
