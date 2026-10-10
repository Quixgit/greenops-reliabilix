package domain

import (
	"testing"
	"time"
)

var k8sNow = time.Date(2026, 10, 10, 0, 0, 0, 0, time.UTC)

func w(mut func(*K8sWorkloadUsage)) K8sWorkloadUsage {
	u := K8sWorkloadUsage{Cluster: "prod", Namespace: "shop", Workload: "checkout", Kind: "deployment", Hours: 300,
		AvgCPUUtilPct: 12, PeakCPUCores: 0.4, CPURequestCores: 2, PeakMemBytes: 600e6, MemRequestBytes: 4 * 1024 * 1024 * 1024}
	if mut != nil {
		mut(&u)
	}
	return u
}

func TestOversizedWorkloadGetsAConservativeRecommendation(t *testing.T) {
	got := GenerateK8sRightsizing("p", []K8sWorkloadUsage{w(nil)}, k8sNow)
	if len(got) != 1 {
		t.Fatalf("%+v", got)
	}
	r := got[0]
	// peak 0.4 cores x 1.25 = 0.5 cores; the request is 2: recommended 0.50.
	if r.Details["cpu_request_recommended_cores"] != "0.50" || r.Details["cpu_request_current_cores"] != "2.00" {
		t.Errorf("cpu: %v", r.Details)
	}
	if r.Type != K8sRightsizing || r.Status != Open || r.CostImpact != nil || r.CarbonReductionKg != 0 || r.CostBasis != "not_estimated" {
		t.Errorf("no saving may be claimed: %+v", r)
	}
	if !r.ComplianceOK() {
		t.Error("a Kubernetes request change has no residency question: it must be approvable")
	}
	if r.CurrentRegion != "prod" || r.ServiceName != "shop/checkout" {
		t.Errorf("place and name feed the dedupe key: %+v", r)
	}
	if r.Confidence <= 0.7 || r.Confidence > 0.85 {
		t.Errorf("300 of 336 hours: confidence %v", r.Confidence)
	}
}

func TestNothingIsProposedWithoutEvidence(t *testing.T) {
	for name, u := range map[string]K8sWorkloadUsage{
		"too few hours":      w(func(u *K8sWorkloadUsage) { u.Hours = 71 }),
		"already tight":      w(func(u *K8sWorkloadUsage) { u.PeakCPUCores = 1.4; u.PeakMemBytes = 3 * 1024 * 1024 * 1024 }),
		"no request":         w(func(u *K8sWorkloadUsage) { u.CPURequestCores = 0; u.MemRequestBytes = 0 }),
		"a job is not sized": w(func(u *K8sWorkloadUsage) { u.Kind = "job" }),
	} {
		if got := GenerateK8sRightsizing("p", []K8sWorkloadUsage{u}, k8sNow); len(got) != 0 {
			t.Errorf("%s must produce nothing: %+v", name, got)
		}
	}
}

func TestOnlyTheOversizedResourceIsChanged(t *testing.T) {
	// CPU is tight, memory is far above its peak.
	got := GenerateK8sRightsizing("p", []K8sWorkloadUsage{w(func(u *K8sWorkloadUsage) { u.PeakCPUCores = 1.6 })}, k8sNow)
	if len(got) != 1 {
		t.Fatalf("%+v", got)
	}
	if _, has := got[0].Details["cpu_request_recommended_cores"]; has {
		t.Errorf("cpu must be left alone: %v", got[0].Details)
	}
	if got[0].Details["memory_request_recommended_bytes"] == "" {
		t.Errorf("memory must be recommended: %v", got[0].Details)
	}
}

func TestRecommendedCPUHasAFloor(t *testing.T) {
	got := GenerateK8sRightsizing("p", []K8sWorkloadUsage{w(func(u *K8sWorkloadUsage) { u.PeakCPUCores = 0.001; u.MemRequestBytes = 0 })}, k8sNow)
	if len(got) != 1 || got[0].Details["cpu_request_recommended_cores"] != "0.05" {
		t.Fatalf("%+v", got)
	}
}
