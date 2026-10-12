// Package azureexport normalizes the daily aggregate that the Azure provider builds from a customer's Cost
// Management export (FOCUS 1.0 cost and usage, CSV) into FOCUS records.
//
// The provider streams the (potentially huge) export and sends only this compact aggregate, so the contract is
// the JSON below. It is deliberately a copy of the provider's struct: the two modules never import each other.
//
// Besides the cost lines the aggregate carries two measurements that Azure bills exactly:
//
//   - vCPU hours of virtual machines (hours x the SKU's core count). The provider files the cost lines they belong
//     to under "Virtual Machines - Compute", so the carbon engine replaces exactly those lines and nothing else:
//     licences, reservations purchases and the like keep their cost-based estimate.
//   - GB-months of Blob Storage data stored, filed under "Storage - Capacity" in the same way.
//
// Memory is not measured: the export carries no memory size per SKU, so the energy of RAM is not part of the VM
// figure (a conservative undercount, stated in the methodology).
package azureexport

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/focus"
)

const (
	unitVCPUHours = "vCPU-Hrs"
	unitStorage   = "GB-Mo"

	svcCompute = "Virtual Machines - Compute"
	svcStorage = "Storage - Capacity"
)

type measure struct {
	Day    string  `json:"day"`
	Region string  `json:"region"`
	Value  float64 `json:"value"`
}

type payload struct {
	Cost []struct {
		Day       string  `json:"day"`
		Service   string  `json:"service"`
		Category  string  `json:"category"`
		Region    string  `json:"region"`
		Currency  string  `json:"currency"`
		Billed    float64 `json:"billed"`
		Effective float64 `json:"effective"`
	} `json:"cost"`
	Tagged     []focus.TagRow `json:"tagged"`
	VCPUHours  []measure      `json:"vm_vcpu_hours"`
	StorageGBM []measure      `json:"storage_gb_months"`
}

type Normalizer struct{}

// Normalize converts the aggregate into cost records plus zero-cost measured records.
func (Normalizer) Normalize(raw focus.RawBatch) ([]focus.Record, error) {
	var p payload
	if err := json.Unmarshal(raw.Payload, &p); err != nil {
		return nil, fmt.Errorf("azureexport: decode payload: %w", err)
	}
	var out []focus.Record
	for _, c := range p.Cost {
		start, end, err := day(c.Day)
		if err != nil {
			return nil, err
		}
		if c.Billed == 0 && c.Effective == 0 {
			continue // nothing charged: no row
		}
		out = append(out, focus.Record{
			Provider: "azure", Source: raw.Source, ServiceName: c.Service, ServiceCategory: Category(c.Category),
			RegionID: region(c.Region), BilledCost: c.Billed, EffectiveCost: c.Effective, Currency: strings.ToUpper(strings.TrimSpace(c.Currency)),
			ChargePeriodStart: start, ChargePeriodEnd: end,
		})
	}
	for _, m := range []struct {
		rows          []measure
		service, unit string
		category      string
	}{{p.VCPUHours, svcCompute, unitVCPUHours, "compute"}, {p.StorageGBM, svcStorage, unitStorage, "storage"}} {
		for _, r := range m.rows {
			start, end, err := day(r.Day)
			if err != nil {
				return nil, err
			}
			if r.Value <= 0 {
				continue
			}
			q, unit := r.Value, m.unit
			out = append(out, focus.Record{
				Provider: "azure", Source: raw.Source, ServiceName: m.service, ServiceCategory: m.category, RegionID: region(r.Region),
				ConsumedQuantity: &q, ConsumedUnit: &unit, Currency: "USD", ChargePeriodStart: start, ChargePeriodEnd: end,
			})
		}
	}
	return out, nil
}

func day(s string) (time.Time, time.Time, error) {
	d, err := time.Parse(time.DateOnly, s)
	if err != nil {
		return d, d, fmt.Errorf("azureexport: bad day %q", s)
	}
	return d, d.AddDate(0, 0, 1), nil
}

// region maps the export's missing, global or zone-only values to "global" (no grid to attribute carbon to).
// Azure regions are lower-case identifiers without separators ("westeurope", "eastus2").
func region(v string) string {
	r := strings.ToLower(strings.TrimSpace(v))
	switch {
	case r == "", r == "global", r == "unassigned", r == "n/a", r == "unknown", strings.HasPrefix(r, "zone "):
		return "global"
	}
	return strings.ReplaceAll(r, " ", "")
}

// Category maps a FOCUS ServiceCategory to the product's reduced category set.
func Category(focusCategory string) string {
	switch strings.ToLower(strings.TrimSpace(focusCategory)) {
	case "compute":
		return "compute"
	case "storage":
		return "storage"
	case "databases":
		return "database"
	case "networking":
		return "networking"
	}
	return "other"
}

// NormalizeTags implements focus.TagNormalizer (values are lower case: Azure's tag values are case-insensitive).
func (Normalizer) NormalizeTags(raw focus.RawBatch) ([]focus.TaggedCost, error) {
	var p payload
	if err := json.Unmarshal(raw.Payload, &p); err != nil {
		return nil, fmt.Errorf("azureexport: decode payload: %w", err)
	}
	out := make([]focus.TaggedCost, 0, len(p.Tagged))
	for _, t := range p.Tagged {
		d, _, err := day(t.Day)
		if err != nil {
			return nil, err
		}
		out = append(out, focus.TaggedCost{Provider: "azure", ServiceName: t.Service, Currency: strings.ToUpper(strings.TrimSpace(t.Currency)), Day: d,
			TagKey: t.Key, TagValue: t.Value, BilledCost: t.Billed, EffectiveCost: t.Effective})
	}
	return out, nil
}
