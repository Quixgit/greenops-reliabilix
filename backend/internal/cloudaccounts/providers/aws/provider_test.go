package aws

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/costexplorer"
	"github.com/aws/aws-sdk-go-v2/service/costexplorer/types"
	"github.com/aws/smithy-go"

	"github.com/quixgit/greenops-reliabilix/backend/internal/cloudaccounts/domain"
)

type fakeCE struct {
	calls []*costexplorer.GetCostAndUsageInput
	pages []*costexplorer.GetCostAndUsageOutput
	err   error
}

func (f *fakeCE) GetCostAndUsage(_ context.Context, in *costexplorer.GetCostAndUsageInput, _ ...func(*costexplorer.Options)) (*costexplorer.GetCostAndUsageOutput, error) {
	cp := *in
	f.calls = append(f.calls, &cp)
	if f.err != nil {
		return nil, f.err
	}
	p := f.pages[0]
	f.pages = f.pages[1:]
	return p, nil
}

func page(day string, next string) *costexplorer.GetCostAndUsageOutput {
	out := &costexplorer.GetCostAndUsageOutput{ResultsByTime: []types.ResultByTime{{
		TimePeriod: &types.DateInterval{Start: awssdk.String(day), End: awssdk.String("2026-09-02")},
		Groups: []types.Group{{Keys: []string{"Amazon Elastic Compute Cloud - Compute", "eu-central-1"},
			Metrics: map[string]types.MetricValue{"UnblendedCost": {Amount: awssdk.String("12.5"), Unit: awssdk.String("USD")}}}},
	}}}
	if next != "" {
		out.NextPageToken = awssdk.String(next)
	}
	return out
}

func conn() domain.Connection {
	return domain.Connection{CredentialRef: "arn:aws:iam::123456789012:role/ReliabilixR", ExternalID: "rlx-abc", AccountRef: "123456789012"}
}

func TestGetUsagePaginatesAndShapesRequest(t *testing.T) {
	ce := &fakeCE{pages: []*costexplorer.GetCostAndUsageOutput{page("2026-09-01", "tok"), page("2026-09-01", "")}}
	var gotRole, gotExt string
	p := &Provider{Assume: func(_ context.Context, role, ext string) (CEAPI, error) { gotRole, gotExt = role, ext; return ce, nil }}
	from := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	batches, err := p.GetUsage(context.Background(), domain.UsageRequest{Connection: conn(), From: from, To: from.AddDate(0, 0, 7)})
	if err != nil {
		t.Fatal(err)
	}
	if gotRole != conn().CredentialRef || gotExt != "rlx-abc" {
		t.Errorf("assume args wrong: %q %q", gotRole, gotExt)
	}
	if len(ce.calls) != 2 || ce.calls[1].NextPageToken == nil || *ce.calls[1].NextPageToken != "tok" {
		t.Fatalf("pagination not followed: %d calls", len(ce.calls))
	}
	in := ce.calls[0]
	if *in.TimePeriod.Start != "2026-09-01" || *in.TimePeriod.End != "2026-09-08" || in.Granularity != types.GranularityDaily || len(in.GroupBy) != 2 {
		t.Errorf("request shape wrong: %+v", in)
	}
	var env struct {
		ResultsByTime []struct {
			TimePeriod struct{ Start, End string }
			Groups     []struct {
				Keys    []string
				Metrics map[string]struct{ Amount, Unit string }
			}
		}
	}
	if err := json.Unmarshal(batches[0].Payload, &env); err != nil || len(env.ResultsByTime) != 2 {
		t.Fatalf("payload: %v %s", err, batches[0].Payload)
	}
	if g := env.ResultsByTime[0].Groups[0]; g.Keys[1] != "eu-central-1" || g.Metrics["UnblendedCost"].Amount != "12.5" {
		t.Errorf("payload content wrong: %+v", g)
	}
	if batches[0].Provider != "aws" || batches[0].Source != SourceCostExplorer {
		t.Errorf("batch metadata: %+v", batches[0])
	}
}

type apiErr struct{ code string }

func (e apiErr) Error() string                 { return e.code }
func (e apiErr) ErrorCode() string             { return e.code }
func (e apiErr) ErrorMessage() string          { return e.code }
func (e apiErr) ErrorFault() smithy.ErrorFault { return smithy.FaultClient }

func TestAccessDeniedIsClassified(t *testing.T) {
	p := &Provider{Assume: func(context.Context, string, string) (CEAPI, error) {
		return &fakeCE{err: apiErr{"AccessDeniedException"}}, nil
	}}
	if err := p.Validate(context.Background(), conn()); !errors.Is(err, domain.ErrAccessDenied) {
		t.Errorf("Validate err = %v", err)
	}
	if _, err := p.GetUsage(context.Background(), domain.UsageRequest{Connection: conn(), From: time.Now(), To: time.Now()}); !errors.Is(err, domain.ErrAccessDenied) {
		t.Errorf("GetUsage err = %v", err)
	}
	other := &Provider{Assume: func(context.Context, string, string) (CEAPI, error) { return &fakeCE{err: errors.New("timeout")}, nil }}
	if err := other.Validate(context.Background(), conn()); errors.Is(err, domain.ErrAccessDenied) || err == nil {
		t.Errorf("transient error misclassified: %v", err)
	}
}
