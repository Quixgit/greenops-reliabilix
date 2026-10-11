// Package awsce normalizes AWS Cost Explorer GetCostAndUsage payloads (grouped by SERVICE and REGION,
// daily) into FOCUS records. It parses the JSON envelope the AWS provider archives; it does not import the SDK.
package awsce

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/focus"
)

type envelope struct {
	ResultsByTime []struct {
		TimePeriod struct{ Start, End string }
		Estimated  bool
		Groups     []struct {
			Keys    []string
			Metrics map[string]struct{ Amount, Unit string }
		}
	}
}

type Normalizer struct{}

// Normalize converts a payload into FOCUS records. Periods AWS still marks as estimated are skipped:
// they are final one sync later, and the sync window overlaps the most recent days.
func (Normalizer) Normalize(raw focus.RawBatch) ([]focus.Record, error) {
	var env envelope
	if err := json.Unmarshal(raw.Payload, &env); err != nil {
		return nil, fmt.Errorf("awsce: decode payload: %w", err)
	}
	// Several raw regions map to one ("NoRegion", "global", "aws global" are all "global"): one record per final key,
	// or the rows would overwrite each other when stored.
	type key struct {
		start, end, service, region, currency string
	}
	type sums struct{ billed, effective float64 }
	merged := map[key]*sums{}
	var order []key
	for _, res := range env.ResultsByTime {
		if res.Estimated {
			continue
		}
		start, err := time.Parse(time.DateOnly, res.TimePeriod.Start)
		if err != nil {
			return nil, fmt.Errorf("awsce: bad period start %q", res.TimePeriod.Start)
		}
		end, err := time.Parse(time.DateOnly, res.TimePeriod.End)
		if err != nil || !end.After(start) {
			return nil, fmt.Errorf("awsce: bad period end %q", res.TimePeriod.End)
		}
		for _, g := range res.Groups {
			if len(g.Keys) != 2 {
				return nil, fmt.Errorf("awsce: group has %d keys, want SERVICE and REGION", len(g.Keys))
			}
			unblended, ok := g.Metrics["UnblendedCost"]
			if !ok {
				return nil, fmt.Errorf("awsce: group %q lacks UnblendedCost", g.Keys[0])
			}
			billed, err := strconv.ParseFloat(unblended.Amount, 64)
			if err != nil {
				return nil, fmt.Errorf("awsce: bad amount %q", unblended.Amount)
			}
			effective := billed
			if am, ok := g.Metrics["AmortizedCost"]; ok {
				if v, err := strconv.ParseFloat(am.Amount, 64); err == nil {
					effective = v
				}
			}
			k := key{res.TimePeriod.Start, res.TimePeriod.End, g.Keys[0], regionOf(g.Keys[1]), strings.ToUpper(unblended.Unit)}
			if merged[k] == nil {
				merged[k] = &sums{}
				order = append(order, k)
			}
			merged[k].billed += billed
			merged[k].effective += effective
		}
	}
	out := make([]focus.Record, 0, len(order))
	for _, k := range order {
		s := merged[k]
		if s.billed == 0 && s.effective == 0 {
			continue // nothing charged: no row, keeps the table small
		}
		start, _ := time.Parse(time.DateOnly, k.start) // validated above
		end, _ := time.Parse(time.DateOnly, k.end)
		out = append(out, focus.Record{
			Provider: "aws", Source: raw.Source,
			ServiceName: k.service, ServiceCategory: CategoryFor(k.service), RegionID: k.region,
			BilledCost: s.billed, EffectiveCost: s.effective, Currency: k.currency,
			ChargePeriodStart: start, ChargePeriodEnd: end,
		})
	}
	return out, nil
}

// regionOf maps Cost Explorer's non-region values to "global".
func regionOf(v string) string {
	switch strings.ToLower(v) {
	case "", "noregion", "global", "aws global":
		return "global"
	}
	return strings.ToLower(v)
}

var categoryRules = []struct {
	category string
	contains []string
}{
	// "EC2 - Other" mixes EBS, NAT gateways and data transfer: it is not compute.
	{"other", []string{"ec2 - other"}},
	{"compute", []string{"elastic compute cloud", "lambda", "elastic container", "fargate", "lightsail", "aws batch", "elastic kubernetes", "app runner", "elastic beanstalk"}},
	{"storage", []string{"simple storage service", "elastic block store", "elastic file system", "glacier", "aws backup", "fsx", "storage gateway"}},
	{"database", []string{"relational database", "dynamodb", "elasticache", "redshift", "documentdb", "neptune", "memorydb", "opensearch", "elasticsearch", "timestream"}},
	{"networking", []string{"virtual private cloud", "cloudfront", "route 53", "elastic load balancing", "direct connect", "api gateway", "global accelerator", "vpn", "transit gateway"}},
}

// CategoryFor maps an AWS service name to the reduced FOCUS ServiceCategory set used by the product.
func CategoryFor(service string) string {
	s := strings.ToLower(service)
	for _, rule := range categoryRules {
		for _, c := range rule.contains {
			if strings.Contains(s, c) {
				return rule.category
			}
		}
	}
	return "other"
}
