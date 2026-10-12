package aws

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/costexplorer"
	"github.com/aws/aws-sdk-go-v2/service/costexplorer/types"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	"github.com/quixgit/greenops-reliabilix/backend/internal/cloudaccounts/domain"
)

// A result that never ends must be an ERROR: a quiet cut-off would let the sync cursor advance past costs that
// were never read, and the gap would stay forever.
func TestCostExplorerPaginationLimitIsAnErrorNotATruncation(t *testing.T) {
	endless := &endlessCE{}
	_, err := fetchAll(context.Background(), endless, &costexplorer.GetCostAndUsageInput{})
	if !errors.Is(err, errTooManyPages) {
		t.Fatalf("err = %v, want errTooManyPages", err)
	}
	if endless.calls != maxCEPages {
		t.Errorf("made %d requests, want exactly the cap (%d): every page costs money", endless.calls, maxCEPages)
	}
}

type endlessCE struct {
	fakeCE // the commitment calls are never made by what this fake is used for
	calls  int
}

func (e *endlessCE) GetCostAndUsage(context.Context, *costexplorer.GetCostAndUsageInput, ...func(*costexplorer.Options)) (*costexplorer.GetCostAndUsageOutput, error) {
	e.calls++
	return &costexplorer.GetCostAndUsageOutput{NextPageToken: awssdk.String("more")}, nil
}
func (e *endlessCE) GetRightsizingRecommendation(context.Context, *costexplorer.GetRightsizingRecommendationInput, ...func(*costexplorer.Options)) (*costexplorer.GetRightsizingRecommendationOutput, error) {
	return nil, nil
}

func controlPage(day string, estimated bool, amount, unit string) types.ResultByTime {
	return types.ResultByTime{
		TimePeriod: &types.DateInterval{Start: awssdk.String(day), End: awssdk.String("2026-09-30")},
		Estimated:  estimated,
		Total:      map[string]types.MetricValue{"UnblendedCost": {Amount: awssdk.String(amount), Unit: awssdk.String(unit)}},
	}
}

func TestControlTotalsAreTheCloudsOwnDayTotals(t *testing.T) {
	ce := &fakeCE{pages: []*costexplorer.GetCostAndUsageOutput{{ResultsByTime: []types.ResultByTime{
		controlPage("2026-09-01", false, "100.5", "USD"),
		controlPage("2026-09-02", false, "-3.25", "USD"), // credits can make a day negative
		controlPage("2026-09-03", true, "9", "USD"),      // estimated: the ingestion skips it, so must the control
	}}}}
	p := &Provider{Assume: func(context.Context, string, string) (CEAPI, error) { return ce, nil }}
	got, err := p.GetControlTotals(context.Background(), domain.UsageRequest{Connection: conn(), From: time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC), To: time.Date(2026, 9, 4, 0, 0, 0, 0, time.UTC)})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].Day != "2026-09-01" || got[0].Billed != 100.5 || got[1].Billed != -3.25 || got[0].Currency != "USD" {
		t.Errorf("control totals = %+v", got)
	}
	in := ce.calls[0]
	if len(in.GroupBy) != 0 || len(in.Metrics) != 1 || in.Metrics[0] != "UnblendedCost" {
		t.Errorf("the control query must be ungrouped UnblendedCost, got %+v", in)
	}
}

func TestControlTotalsRefuseGarbageAndSkipExports(t *testing.T) {
	ce := &fakeCE{pages: []*costexplorer.GetCostAndUsageOutput{{ResultsByTime: []types.ResultByTime{controlPage("2026-09-01", false, "NaN", "USD")}}}}
	p := &Provider{Assume: func(context.Context, string, string) (CEAPI, error) { return ce, nil }}
	req := domain.UsageRequest{Connection: conn(), From: time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC), To: time.Date(2026, 9, 2, 0, 0, 0, 0, time.UTC)}
	if _, err := p.GetControlTotals(context.Background(), req); err == nil {
		t.Error("a NaN control total must be refused")
	}
	req.Connection = exportConn()
	if got, err := p.GetControlTotals(context.Background(), req); err != nil || got != nil {
		t.Errorf("export connections have no CE control (got %v, %v)", got, err)
	}
}

func TestLineLimitStopsABufferEverythingFile(t *testing.T) {
	r := &lineLimited{r: strings.NewReader(strings.Repeat("a", maxLineBytes+10))}
	if _, err := io.ReadAll(r); !errors.Is(err, errLongLine) {
		t.Errorf("err = %v, want errLongLine", err)
	}
	ok := &lineLimited{r: strings.NewReader(strings.Repeat(strings.Repeat("a", 1000)+"\n", 5000))} // many short lines are fine
	if _, err := io.ReadAll(ok); err != nil {
		t.Errorf("short lines must pass: %v", err)
	}
}

func TestTotalByteBudgetIsSharedAcrossFiles(t *testing.T) {
	budget := int64(100)
	a := &limitedReader{r: strings.NewReader(strings.Repeat("x", 80)), n: 1000, total: &budget}
	if _, err := io.ReadAll(a); err != nil {
		t.Fatalf("first file is within the budget: %v", err)
	}
	b := &limitedReader{r: strings.NewReader(strings.Repeat("x", 80)), n: 1000, total: &budget}
	if _, err := io.ReadAll(b); !errors.Is(err, errTooLarge) {
		t.Errorf("a second file must exhaust the shared budget, got %v", err)
	}
}

// Every S3 read is pinned to the connection's own AWS account.
type ownerS3 struct{ owners []string }

func (o *ownerS3) GetObject(_ context.Context, in *s3.GetObjectInput, _ ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
	if in.ExpectedBucketOwner == nil {
		o.owners = append(o.owners, "")
	} else {
		o.owners = append(o.owners, *in.ExpectedBucketOwner)
	}
	return &s3.GetObjectOutput{Body: io.NopCloser(bytes.NewReader([]byte(`{"dataFiles":[]}`)))}, nil
}

func TestS3ReadsAreBoundToTheOwningAccount(t *testing.T) {
	o := &ownerS3{}
	p := &Provider{AssumeS3: func(context.Context, string, string, string) (S3API, error) { return o, nil }}
	_ = p.Validate(context.Background(), exportConn())
	if len(o.owners) == 0 {
		t.Fatal("no S3 request was made")
	}
	for _, owner := range o.owners {
		if owner != "123456789012" {
			t.Errorf("S3 request without ExpectedBucketOwner pinning: %q", owner)
		}
	}
}
