// Package gcpbilling normalizes the daily aggregate that the GCP provider builds from a customer's Cloud Billing
// export (BigQuery) into FOCUS records.
//
// The aggregate is one line per day, service, SKU, region, currency and pricing unit. This package decides what
// each SKU is worth in carbon terms:
//
//   - Compute Engine core and RAM SKUs carry exact vCPU-hours and GiB-hours: they become measured usage records.
//   - Persistent-disk capacity and Cloud Storage capacity SKUs carry GiB-months: measured as well.
//   - Everything else stays a cost line (cost-based estimate).
//
// Measured SKUs are filed under their own service names ("Compute Engine - Compute", "Compute Engine - Disk",
// "Cloud Storage - Capacity") so the carbon engine replaces exactly the cost lines they correspond to and nothing
// else: GPUs, networking or licences of the same service keep their cost-based estimate instead of vanishing.
package gcpbilling

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/focus"
)

// gibToGB converts the binary units GCP bills in (gibibyte) to the decimal gigabytes of the methodology.
const gibToGB = 1.073741824

const (
	unitVCPUHours  = "vCPU-Hrs"
	unitMemoryHour = "GB-Hrs"
	unitStorage    = "GB-Mo"

	svcCompute           = "Compute Engine - Compute"
	svcDisk              = "Compute Engine - Disk"
	svcAccel             = "Compute Engine - Accelerators"
	svcComputeStor       = "Compute Engine - Storage"
	svcComputeNet        = "Compute Engine - Network"
	svcComputeOth        = "Compute Engine - Other"
	svcComputeUnmeasured = "Compute Engine - Other compute"
	svcGCSCapacity       = "Cloud Storage - Capacity"
)

type line struct {
	Day      string `json:"day"`
	Service  string `json:"service"`
	SKU      string `json:"sku"`
	Region   string `json:"region"`
	Currency string `json:"currency"`
	Unit     string `json:"pricing_unit"`
	Cost     string `json:"cost"`
	Credits  string `json:"credits"`
	Quantity string `json:"quantity"`
}

type Normalizer struct{}

type measure int

const (
	none measure = iota
	vcpuHours
	memoryHours
	storageMonths
)

var (
	coreRe        = regexp.MustCompile(`(?i)\binstance core running\b`)
	ramRe         = regexp.MustCompile(`(?i)\binstance ram running\b`)
	soleTenantRe  = regexp.MustCompile(`(?i)sole tenan`)
	acceleratorRe = regexp.MustCompile(`(?i)\b(gpu|tpu)\b`)
	pdCapacityRe  = regexp.MustCompile(`(?i)\bpd capacity\b`)
	computeStorRe = regexp.MustCompile(`(?i)(storage pd|snapshot|\bimage\b|local ssd|ssd backed local)`)
	networkRe     = regexp.MustCompile(`(?i)(network|egress|ingress|\bcdn\b|\bnat\b|load balanc|ip address)`)
	commitmentRe  = regexp.MustCompile(`(?i)^commitment`)
	gcsNotCapRe   = regexp.MustCompile(`(?i)(operation|retrieval|early delete|egress|network|replication|class [ab]\b|management)`)
	storageWordRe = regexp.MustCompile(`(?i)\bstorage\b`)
)

// classify returns the service name and category a line is filed under and how (if at all) it is measured.
func classify(service, sku, unit string) (string, string, measure) {
	unit = strings.ToLower(strings.TrimSpace(unit))
	switch service {
	case "Compute Engine":
		switch {
		case coreRe.MatchString(sku) && !soleTenantRe.MatchString(sku) && unit == "hour":
			return svcCompute, "compute", vcpuHours
		case ramRe.MatchString(sku) && !soleTenantRe.MatchString(sku) && unit == "gibibyte hour":
			return svcCompute, "compute", memoryHours
		case acceleratorRe.MatchString(sku):
			return svcAccel, "compute", none
		case pdCapacityRe.MatchString(sku) && unit == "gibibyte month":
			return svcDisk, "storage", storageMonths
		case pdCapacityRe.MatchString(sku):
			return svcDisk, "storage", none
		case computeStorRe.MatchString(sku):
			return svcComputeStor, "storage", none
		case networkRe.MatchString(sku):
			return svcComputeNet, "networking", none
		case commitmentRe.MatchString(sku):
			return svcCompute, "compute", none // a fee for reserved capacity: the usage lines carry the energy
		case coreRe.MatchString(sku), ramRe.MatchString(sku):
			// core/RAM lines that cannot be measured (sole tenancy, a changed unit) keep their own cost-based
			// estimate: filed under the measured service name they would be replaced by an undercount
			return svcComputeUnmeasured, "compute", none
		}
		return svcComputeOth, "other", none
	case "Cloud Storage":
		if unit == "gibibyte month" && storageWordRe.MatchString(sku) && !gcsNotCapRe.MatchString(sku) {
			return svcGCSCapacity, "storage", storageMonths
		}
		return service, "storage", none
	}
	return service, categoryOf(service), none
}

var categories = map[string]string{}

func init() {
	for category, names := range map[string][]string{
		"compute": {"Kubernetes Engine", "Cloud Run", "Cloud Run Functions", "Cloud Functions", "App Engine", "Batch", "Dataproc", "Dataflow", "Vertex AI"},
		"storage": {"Filestore", "Backup and DR Service", "Artifact Registry"},
		"database": {"Cloud SQL", "Cloud Spanner", "Firestore", "Cloud Bigtable", "AlloyDB", "BigQuery", "Datastore",
			"Memorystore for Redis", "Memorystore for Memcached", "Memorystore for Valkey"},
		"networking": {"Networking", "Cloud DNS", "Cloud CDN", "Cloud Load Balancing", "Cloud NAT", "Cloud Armor", "Cloud VPN",
			"Cloud Interconnect", "Network Connectivity", "Network Intelligence Center", "Cloud Router"},
	} {
		for _, n := range names {
			categories[n] = category
		}
	}
}

func categoryOf(service string) string {
	if c, ok := categories[service]; ok {
		return c
	}
	return "other"
}

type costKey struct{ day, service, category, region, currency string }
type measureKey struct {
	day, region, service, category string
	kind                           measure
}

// Normalize converts the aggregate into cost records and zero-cost measured records.
func (Normalizer) Normalize(raw focus.RawBatch) ([]focus.Record, error) {
	var p struct {
		Rows []line `json:"rows"`
	}
	if err := json.Unmarshal(raw.Payload, &p); err != nil {
		return nil, fmt.Errorf("gcpbilling: decode payload: %w", err)
	}
	type sums struct{ billed, effective float64 }
	costs := map[costKey]*sums{}
	qty := map[measureKey]float64{}
	for _, l := range p.Rows {
		day, err := time.Parse(time.DateOnly, l.Day)
		if err != nil {
			return nil, fmt.Errorf("gcpbilling: bad day %q", l.Day)
		}
		cost, err1 := strconv.ParseFloat(l.Cost, 64)
		credits, err2 := strconv.ParseFloat(l.Credits, 64)
		if err1 != nil || err2 != nil {
			return nil, fmt.Errorf("gcpbilling: bad amount on %s %q", l.Day, l.Service)
		}
		service, category, m := classify(l.Service, l.SKU, l.Unit)
		region := regionOf(l.Region)
		k := costKey{day.Format(time.DateOnly), service, category, region, strings.ToUpper(strings.TrimSpace(l.Currency))}
		if costs[k] == nil {
			costs[k] = &sums{}
		}
		costs[k].billed += cost
		costs[k].effective += cost + credits // credits are negative amounts
		if m != none {
			if q, err := strconv.ParseFloat(l.Quantity, 64); err == nil && q > 0 {
				qty[measureKey{day.Format(time.DateOnly), region, service, category, m}] += q
			}
		}
	}

	var out []focus.Record
	for k, s := range costs {
		if s.billed == 0 && s.effective == 0 {
			continue // nothing charged: no row
		}
		start, _ := time.Parse(time.DateOnly, k.day)
		out = append(out, focus.Record{
			Provider: "gcp", Source: raw.Source, ServiceName: k.service, ServiceCategory: k.category, RegionID: k.region,
			BilledCost: s.billed, EffectiveCost: s.effective, Currency: k.currency,
			ChargePeriodStart: start, ChargePeriodEnd: start.AddDate(0, 0, 1),
		})
	}
	for k, q := range qty {
		start, _ := time.Parse(time.DateOnly, k.day)
		out = append(out, measuredRecord(raw.Source, k, q, start))
	}
	return out, nil
}

func measuredRecord(source string, k measureKey, q float64, start time.Time) focus.Record {
	unit := unitVCPUHours
	switch k.kind {
	case memoryHours:
		unit, q = unitMemoryHour, q*gibToGB
	case storageMonths:
		unit, q = unitStorage, q*gibToGB
	}
	return focus.Record{
		Provider: "gcp", Source: source, ServiceName: k.service, ServiceCategory: k.category, RegionID: k.region,
		ConsumedQuantity: &q, ConsumedUnit: &unit, Currency: "USD",
		ChargePeriodStart: start, ChargePeriodEnd: start.AddDate(0, 0, 1),
	}
}

// regionOf maps missing or global locations to "global" (no grid to attribute carbon to).
func regionOf(v string) string {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "", "global":
		return "global"
	}
	return strings.ToLower(strings.TrimSpace(v))
}
