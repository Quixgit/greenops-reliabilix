// Package awsexport normalizes the daily aggregate that the AWS provider builds from a customer's FOCUS data
// export (AWS Data Exports, FOCUS 1.0 columns) into FOCUS records.
//
// The provider streams the (potentially huge) export and sends only this compact aggregate, so the contract is
// the JSON below. It is deliberately a copy of the provider's struct: the two modules never import each other.
//
// Compared with Cost Explorer the export is the invoice-level source of truth: it has FOCUS service categories
// (EBS counts as storage, NAT as networking) and exact instance hours per instance type. The provider names
// EC2 charges like Cost Explorer does ("... - Compute" for running hours, "EC2 - Other" for the rest), so the
// carbon engine can match measured hours to exactly the cost line they replace.
package awsexport

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/quixgit/greenops-reliabilix/backend/internal/ingestion/normalizers/awsceusage"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/focus"
)

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
	EC2Hours []struct {
		Day          string  `json:"day"`
		Region       string  `json:"region"`
		InstanceType string  `json:"instance_type"`
		Hours        float64 `json:"hours"`
	} `json:"ec2_hours"`
	S3GBMonths []struct {
		Day      string  `json:"day"`
		Region   string  `json:"region"`
		GBMonths float64 `json:"gb_months"`
	} `json:"s3_gb_months"`
}

type Normalizer struct{}

// Normalize converts the aggregate into cost records plus zero-cost measured records (see awsceusage).
func (Normalizer) Normalize(raw focus.RawBatch) ([]focus.Record, error) {
	var p payload
	if err := json.Unmarshal(raw.Payload, &p); err != nil {
		return nil, fmt.Errorf("awsexport: decode payload: %w", err)
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
			Provider: "aws", Source: raw.Source, ServiceName: c.Service, ServiceCategory: Category(c.Category),
			RegionID: region(c.Region), BilledCost: c.Billed, EffectiveCost: c.Effective, Currency: strings.ToUpper(c.Currency),
			ChargePeriodStart: start, ChargePeriodEnd: end,
		})
	}

	type dayRegion struct{ day, region string }
	hours := map[dayRegion]map[string]float64{}
	for _, h := range p.EC2Hours {
		k := dayRegion{h.Day, region(h.Region)}
		if hours[k] == nil {
			hours[k] = map[string]float64{}
		}
		hours[k][strings.ToLower(h.InstanceType)] += h.Hours
	}
	byDay := map[string]map[string]map[string]float64{} // day -> region -> instance type -> hours
	for k, v := range hours {
		if byDay[k.day] == nil {
			byDay[k.day] = map[string]map[string]float64{}
		}
		byDay[k.day][k.region] = v
	}
	for d, regions := range byDay {
		start, end, err := day(d)
		if err != nil {
			return nil, err
		}
		out = append(out, awsceusage.EC2Records(raw.Source, start, end, regions)...)
	}
	for _, s := range p.S3GBMonths {
		start, end, err := day(s.Day)
		if err != nil {
			return nil, err
		}
		if s.GBMonths > 0 {
			out = append(out, awsceusage.StorageRecord(raw.Source, region(s.Region), s.GBMonths, start, end))
		}
	}
	return out, nil
}

func day(s string) (time.Time, time.Time, error) {
	d, err := time.Parse(time.DateOnly, s)
	if err != nil {
		return d, d, fmt.Errorf("awsexport: bad day %q", s)
	}
	return d, d.AddDate(0, 0, 1), nil
}

// region maps the export's missing or global region values to "global" (no grid to attribute carbon to).
func region(v string) string {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "", "global", "n/a", "noregion", "aws global":
		return "global"
	}
	return strings.ToLower(strings.TrimSpace(v))
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
