package domain

import (
	"fmt"
	"math"
	"sort"
	"time"
)

// K8sRightsizing is a recommendation to lower the CPU and memory requests of a workload (ADR-0031).
const K8sRightsizing Type = "k8s_rightsizing"

// Rules of the generator. They are deliberately conservative: hourly averages hide spikes, so the recommended request is
// the PEAK hourly usage of the window plus headroom, and a change is proposed only when it frees a large share.
const (
	K8sMinHours        = 72   // hours with both usage and a request in the window (three days)
	K8sCPUHeadroom     = 1.25 // recommended cores = peak hourly usage x this
	K8sMemHeadroom     = 1.25
	K8sMaxShare        = 0.60 // propose only when the recommended request is at most this share of the current one
	K8sMinCPUCores     = 0.05 // never recommend below 50 millicores
	k8sMinMemBytes     = 64 * 1024 * 1024
	k8sFullWindowHours = 336 // two weeks: the hours at which confidence stops growing
)

// K8sWorkloadUsage is what the generator reads about one workload (supplied by the repository).
type K8sWorkloadUsage struct {
	Cluster, Namespace, Workload, Kind string
	Hours                              int // hours that have usage and a request
	AvgCPUUtilPct                      float64
	PeakCPUCores                       float64 // highest hourly average usage, in cores
	CPURequestCores                    float64 // average request
	PeakMemBytes                       float64
	MemRequestBytes                    float64
}

func ceilTo(v, step float64) float64 { return math.Ceil(v/step-1e-9) * step }

func gib(b float64) string { return fmt.Sprintf("%.1f GiB", b/(1024*1024*1024)) }

// GenerateK8sRightsizing returns a recommendation per workload whose requests are far above its measured peak.
// A workload with too few hours, no request, or a request that is already close to its use produces nothing.
func GenerateK8sRightsizing(projectID string, ws []K8sWorkloadUsage, now time.Time) []Recommendation {
	var out []Recommendation
	for _, w := range ws {
		if w.Hours < K8sMinHours || (w.Kind != "deployment" && w.Kind != "statefulset") {
			continue
		}
		var parts []string
		details := map[string]string{"cluster": w.Cluster, "namespace": w.Namespace, "workload": w.Workload, "kind": w.Kind,
			"hours": fmt.Sprint(w.Hours), "method": "peak_hourly_usage_plus_headroom"}
		if w.CPURequestCores > 0 {
			rec := ceilTo(math.Max(w.PeakCPUCores*K8sCPUHeadroom, K8sMinCPUCores), 0.05)
			if rec <= w.CPURequestCores*K8sMaxShare {
				details["cpu_request_current_cores"] = fmt.Sprintf("%.2f", w.CPURequestCores)
				details["cpu_request_recommended_cores"] = fmt.Sprintf("%.2f", rec)
				details["cpu_peak_cores"] = fmt.Sprintf("%.2f", w.PeakCPUCores)
				parts = append(parts, fmt.Sprintf("CPU %.2f → %.2f cores", w.CPURequestCores, rec))
			}
		}
		if w.MemRequestBytes > 0 {
			rec := math.Max(math.Ceil(w.PeakMemBytes*K8sMemHeadroom/(16*1024*1024))*(16*1024*1024), k8sMinMemBytes)
			if rec <= w.MemRequestBytes*K8sMaxShare {
				details["memory_request_current_bytes"] = fmt.Sprintf("%.0f", w.MemRequestBytes)
				details["memory_request_recommended_bytes"] = fmt.Sprintf("%.0f", rec)
				details["memory_peak_bytes"] = fmt.Sprintf("%.0f", w.PeakMemBytes)
				parts = append(parts, fmt.Sprintf("memory %s → %s", gib(w.MemRequestBytes), gib(rec)))
			}
		}
		if len(parts) == 0 {
			continue
		}
		title := fmt.Sprintf("Right-size %s/%s: %s", w.Namespace, w.Workload, joinParts(parts))
		out = append(out, Recommendation{
			Type: K8sRightsizing, Title: title, Provider: "kubernetes", ServiceName: w.Namespace + "/" + w.Workload,
			CurrentRegion: w.Cluster, // the cluster is the "place" of this finding (also part of its dedupe key)
			// The saving is not claimed: smaller requests free node capacity, which only becomes fewer nodes if the scheduler
			// can pack better. Energy and cost impact stay "not estimated" rather than guessed.
			CostBasis: "not_estimated", Confidence: math.Round(math.Min(float64(w.Hours)/k8sFullWindowHours, 1)*0.85*100) / 100,
			Compliance: ComplianceCheck{Residency: "not_applicable", CheckedAt: now}, Status: Open, Details: details,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Title < out[j].Title })
	return out
}

func joinParts(p []string) string {
	s := p[0]
	for _, x := range p[1:] {
		s += ", " + x
	}
	return s
}
