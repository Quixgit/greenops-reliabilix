"use client";

import { useState } from "react";
import { ConnectCluster } from "@/components/kubernetes/connect-cluster";
import { PageHead, Sections } from "@/components/dx/page";
import { Ico } from "@/components/dx/icons";
import { ClusterCards, EnergySplit, K8sSetup, K8sStats, RightSizing, WorkloadTable } from "@/components/kubernetes/k8s-board";
import { K8sCost } from "@/components/kubernetes/k8s-cost";
import { useK8sClusters } from "@/hooks/use-overview";
import { useT } from "@/i18n/provider";

export function KubernetesView({ canConnect = false }: { canConnect?: boolean }) {
  const t = useT();
  const [cluster, setCluster] = useState("");
  const [connecting, setConnecting] = useState(false);
  const clusters = useK8sClusters();
  const none = !clusters.isLoading && !clusters.isError && (clusters.data?.length ?? 0) === 0;
  return (
    <div className="page-enter">
      <PageHead title="Kubernetes" sub={t("k8.subtitle")}
        actions={canConnect && !connecting && <button type="button" className="dx-btn is-primary" onClick={() => setConnecting(true)}><Ico name="plus" size={16} />{t("kc.connect")}</button>} />
      {connecting && <div style={{ marginBottom: 20 }}><ConnectCluster onClose={() => setConnecting(false)} /></div>}
      {none ? (connecting ? null : <K8sSetup onConnect={canConnect ? () => setConnecting(true) : undefined} />) : (
        <Sections label="Kubernetes" tabs={[
          { id: "clusters", label: t("dx.k8s.clusters", undefined, "Clusters"), count: clusters.data?.length ?? null },
          { id: "cost", label: t("dx.k8s.cost", undefined, "Cost and energy") },
          { id: "workloads", label: t("dx.k8s.workloads", undefined, "Workloads and right-sizing") },
        ]}>
          {(tab) => tab === "clusters" ? (
            <>
              <K8sStats />
              <ClusterCards cluster={cluster} onPick={setCluster} />
            </>
          ) : tab === "cost" ? (
            <div className="dxp-split is-even"><K8sCost cluster={cluster} canEdit={canConnect} /><EnergySplit cluster={cluster} /></div>
          ) : (
            <>
              <RightSizing />
              <WorkloadTable cluster={cluster} onClear={() => setCluster("")} />
            </>
          )}
        </Sections>
      )}
    </div>
  );
}
