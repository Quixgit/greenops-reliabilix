// Package awsceusage normalizes the measured-usage payload of the AWS provider (Cost Explorer UsageQuantity
// for EC2 running hours and S3 storage) into FOCUS records that carry ConsumedQuantity/ConsumedUnit.
//
// The records have zero cost on purpose: the charge itself already arrives through the cost payload, and a
// second cost line would double every total. They exist so the carbon engine can use measured energy.
package awsceusage

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/ec2spec"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/focus"
)

const (
	ec2Service = "Amazon Elastic Compute Cloud - Compute"
	s3Service  = "Amazon Simple Storage Service"

	unitVCPUHours  = "vCPU-Hrs"
	unitMemoryHour = "GB-Hrs"
	unitStorage    = "GB-Mo"
)

type group struct {
	Keys    []string
	Metrics map[string]struct{ Amount, Unit string }
}

type results []struct {
	TimePeriod struct{ Start, End string }
	Estimated  bool
	Groups     []group
}

type envelope struct {
	EC2Hours  results
	S3Storage results
}

type Normalizer struct{}

// Normalize converts a payload into FOCUS records. Rows AWS still marks as estimated are skipped (they are
// final one sync later). For EC2, a (region, day) is emitted only when every instance type of that day has a
// known shape: a partial figure would replace the cost-based estimate of the whole service with an undercount.
func (Normalizer) Normalize(raw focus.RawBatch) ([]focus.Record, error) {
	var env envelope
	if err := json.Unmarshal(raw.Payload, &env); err != nil {
		return nil, fmt.Errorf("awsceusage: decode payload: %w", err)
	}
	ec2, err := normalizeEC2(raw.Source, env.EC2Hours)
	if err != nil {
		return nil, err
	}
	s3, err := normalizeS3(raw.Source, env.S3Storage)
	if err != nil {
		return nil, err
	}
	return append(ec2, s3...), nil
}

func period(start, end string) (time.Time, time.Time, error) {
	s, err := time.Parse(time.DateOnly, start)
	if err != nil {
		return s, s, fmt.Errorf("awsceusage: bad period start %q", start)
	}
	e, err := time.Parse(time.DateOnly, end)
	if err != nil || !e.After(s) {
		return s, e, fmt.Errorf("awsceusage: bad period end %q", end)
	}
	return s, e, nil
}

// quantity reads UsageQuantity and verifies its unit, so a changed AWS unit never silently scales a result.
func quantity(g group, wantUnit string) (float64, bool) {
	m, ok := g.Metrics["UsageQuantity"]
	if !ok || !strings.EqualFold(m.Unit, wantUnit) {
		return 0, false
	}
	v, err := strconv.ParseFloat(m.Amount, 64)
	if err != nil || v <= 0 {
		return 0, false
	}
	return v, true
}

func normalizeEC2(source string, in results) ([]focus.Record, error) {
	var out []focus.Record
	for _, res := range in {
		if res.Estimated {
			continue
		}
		start, end, err := period(res.TimePeriod.Start, res.TimePeriod.End)
		if err != nil {
			return nil, err
		}
		type shape struct{ vcpuHours, memHours float64 }
		byRegion := map[string]map[string]shape{} // region -> instance type -> hours-weighted shape
		complete := map[string]bool{}
		for _, g := range res.Groups {
			if len(g.Keys) != 2 {
				return nil, fmt.Errorf("awsceusage: EC2 group has %d keys, want REGION and INSTANCE_TYPE", len(g.Keys))
			}
			region, itype := strings.ToLower(g.Keys[0]), strings.ToLower(g.Keys[1])
			if _, seen := complete[region]; !seen {
				complete[region] = true
			}
			hours, ok := quantity(g, "Hrs")
			if !ok {
				continue // zero or unexpected unit: nothing measurable here
			}
			spec, known := ec2spec.Parse(itype)
			if !known {
				complete[region] = false
				continue
			}
			if byRegion[region] == nil {
				byRegion[region] = map[string]shape{}
			}
			byRegion[region][itype] = shape{vcpuHours: hours * spec.VCPU, memHours: hours * spec.MemoryGB}
		}
		for region, types := range byRegion {
			if !complete[region] {
				continue
			}
			for itype, sh := range types {
				out = append(out,
					measured(source, ec2Service, "compute", region, "instance-type/"+itype, sh.vcpuHours, unitVCPUHours, start, end),
					measured(source, ec2Service, "compute", region, "instance-type/"+itype, sh.memHours, unitMemoryHour, start, end))
			}
		}
	}
	return out, nil
}

func normalizeS3(source string, in results) ([]focus.Record, error) {
	var out []focus.Record
	for _, res := range in {
		if res.Estimated {
			continue
		}
		start, end, err := period(res.TimePeriod.Start, res.TimePeriod.End)
		if err != nil {
			return nil, err
		}
		for _, g := range res.Groups {
			if len(g.Keys) != 1 {
				return nil, fmt.Errorf("awsceusage: S3 group has %d keys, want REGION", len(g.Keys))
			}
			gbMonths, ok := quantity(g, "GB-Mo")
			if !ok {
				continue
			}
			out = append(out, measured(source, s3Service, "storage", strings.ToLower(g.Keys[0]), "", gbMonths, unitStorage, start, end))
		}
	}
	return out, nil
}

func measured(source, service, category, region, resourceID string, qty float64, unit string, start, end time.Time) focus.Record {
	return focus.Record{
		Provider: "aws", Source: source, ResourceID: resourceID, ResourceType: resourceTypeOf(resourceID),
		ServiceName: service, ServiceCategory: category, RegionID: region,
		ConsumedQuantity: &qty, ConsumedUnit: &unit, Currency: "USD",
		ChargePeriodStart: start, ChargePeriodEnd: end,
	}
}

func resourceTypeOf(resourceID string) string {
	if strings.HasPrefix(resourceID, "instance-type/") {
		return "ec2-instance-type"
	}
	return ""
}
