package aws

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
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
	pages []*costexplorer.GetCostAndUsageOutput // served in order; once exhausted, empty results
	err   error
	// failFrom > 0 makes every call from that (1-based) call number on fail with failErr.
	failFrom int
	failErr  error

	rs      []*costexplorer.GetRightsizingRecommendationOutput
	rsCalls []*costexplorer.GetRightsizingRecommendationInput
}

func (f *fakeCE) GetCostAndUsage(_ context.Context, in *costexplorer.GetCostAndUsageInput, _ ...func(*costexplorer.Options)) (*costexplorer.GetCostAndUsageOutput, error) {
	cp := *in
	f.calls = append(f.calls, &cp)
	if f.err != nil {
		return nil, f.err
	}
	if f.failFrom > 0 && len(f.calls) >= f.failFrom {
		return nil, f.failErr
	}
	if len(f.pages) == 0 {
		return &costexplorer.GetCostAndUsageOutput{}, nil
	}
	p := f.pages[0]
	f.pages = f.pages[1:]
	return p, nil
}

// rightsizing responses are served from rs (pages in order); rsCalls records the requests.
func (f *fakeCE) GetRightsizingRecommendation(_ context.Context, in *costexplorer.GetRightsizingRecommendationInput, _ ...func(*costexplorer.Options)) (*costexplorer.GetRightsizingRecommendationOutput, error) {
	cp := *in
	f.rsCalls = append(f.rsCalls, &cp)
	if f.err != nil {
		return nil, f.err
	}
	if len(f.rs) == 0 {
		return &costexplorer.GetRightsizingRecommendationOutput{}, nil
	}
	p := f.rs[0]
	f.rs = f.rs[1:]
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
	if len(ce.calls) != 4 || ce.calls[1].NextPageToken == nil || *ce.calls[1].NextPageToken != "tok" {
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
	if len(batches) != 2 || batches[1].Source != SourceCostExplorerUsage {
		t.Fatalf("want the cost batch plus a usage batch, got %d", len(batches))
	}
}

func TestUsageQueriesAskForRunningHoursAndStorage(t *testing.T) {
	ce := &fakeCE{}
	p := &Provider{Assume: func(context.Context, string, string) (CEAPI, error) { return ce, nil }}
	from := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	if _, err := p.GetUsage(context.Background(), domain.UsageRequest{Connection: conn(), From: from, To: from.AddDate(0, 0, 1)}); err != nil {
		t.Fatal(err)
	}
	if len(ce.calls) != 3 {
		t.Fatalf("calls = %d, want cost + EC2 hours + S3 storage", len(ce.calls))
	}
	ec2, s3 := ce.calls[1], ce.calls[2]
	if got := ec2.Filter.Dimensions.Values; len(got) != 1 || got[0] != "EC2: Running Hours" || len(ec2.GroupBy) != 2 || *ec2.GroupBy[1].Key != "INSTANCE_TYPE" {
		t.Errorf("EC2 query wrong: %+v / %+v", ec2.Filter.Dimensions, ec2.GroupBy)
	}
	if len(s3.GroupBy) != 1 || *s3.GroupBy[0].Key != "REGION" || ec2.Metrics[0] != "UsageQuantity" {
		t.Errorf("S3 query wrong: %+v", s3)
	}
}

func TestUsageFailureDoesNotBlockCostSync(t *testing.T) {
	ce := &fakeCE{pages: []*costexplorer.GetCostAndUsageOutput{page("2026-09-01", "")}, failFrom: 2, failErr: errors.New("throttled")}
	p := &Provider{Assume: func(context.Context, string, string) (CEAPI, error) { return ce, nil }}
	from := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	batches, err := p.GetUsage(context.Background(), domain.UsageRequest{Connection: conn(), From: from, To: from.AddDate(0, 0, 1)})
	if err != nil || len(batches) != 1 || batches[0].Source != SourceCostExplorer {
		t.Fatalf("cost data must survive a usage failure: %v %d", err, len(batches))
	}
}

func TestUsageAccessDeniedFailsTheSync(t *testing.T) {
	ce := &fakeCE{pages: []*costexplorer.GetCostAndUsageOutput{page("2026-09-01", "")}, failFrom: 2, failErr: apiErr{"AccessDeniedException"}}
	p := &Provider{Assume: func(context.Context, string, string) (CEAPI, error) { return ce, nil }}
	from := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	if _, err := p.GetUsage(context.Background(), domain.UsageRequest{Connection: conn(), From: from, To: from.AddDate(0, 0, 1)}); !errors.Is(err, domain.ErrAccessDenied) {
		t.Fatalf("a permission problem must surface, got %v", err)
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

func TestClassifyNamesTheCauseOfAnAwsFailure(t *testing.T) {
	api := func(code, msg string) error { return &smithy.GenericAPIError{Code: code, Message: msg} }
	for _, tc := range []struct {
		name         string
		err          error
		wantDenied   bool
		wantContains string
	}{
		{"assume role refused", api("AccessDenied", "User: arn:aws:sts::1:assumed-role/x is not authorized to perform: sts:AssumeRole"), true, "trust policy"},
		{"cost explorer not enabled", api("AccessDeniedException", "User not enabled for cost explorer access"), true, "enable Cost Explorer"},
		{"missing permission", api("AccessDeniedException", "not allowed"), true, "AccessDeniedException"},
		{"throttled", api("ThrottlingException", "Rate exceeded"), false, "throttling"},
		{"limit exceeded", api("LimitExceededException", "too many"), false, "throttling"},
		{"data unavailable", api("DataUnavailableException", "no data"), false, "24 hours"},
		{"other code", api("InternalServiceError", "boom"), false, "InternalServiceError"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := classify(tc.err)
			var pe domain.PublicError
			if !errors.As(err, &pe) {
				t.Fatalf("not a public error: %v", err)
			}
			if !strings.Contains(pe.Message, tc.wantContains) {
				t.Errorf("message %q lacks %q", pe.Message, tc.wantContains)
			}
			if errors.Is(err, domain.ErrAccessDenied) != tc.wantDenied || domain.NeedsCustomerAction(err) != tc.wantDenied {
				t.Errorf("customer action = %v, want %v", domain.NeedsCustomerAction(err), tc.wantDenied)
			}
		})
	}
	// an error that is not from the AWS API (network, context) is left as it is
	plain := errors.New("dial tcp: i/o timeout")
	if classify(plain) != plain {
		t.Error("a non-API error must pass through")
	}
}

func TestClassifyBlamesThePlatformWhenItHasNoCredentials(t *testing.T) {
	err := classify(errors.New("operation error STS: AssumeRole, get identity: get credentials: failed to refresh cached credentials, no EC2 IMDS role found"))
	var pe domain.PublicError
	if !errors.As(err, &pe) || !strings.Contains(pe.Message, "platform's own AWS identity") || domain.NeedsCustomerAction(err) {
		t.Fatalf("got %v", err)
	}
}

func TestAccessKeyConnectionSignsWithTheOpenedSecretAndNeverAssumesARole(t *testing.T) {
	ce := &fakeCE{pages: []*costexplorer.GetCostAndUsageOutput{page("2026-09-01", "")}}
	var gotID, gotSecret string
	assumed := false
	p := &Provider{
		Assume: func(context.Context, string, string) (CEAPI, error) { assumed = true; return ce, nil },
		Key: func(_ context.Context, id, secret string) (CEAPI, error) {
			gotID, gotSecret = id, secret
			return ce, nil
		},
		OpenSecret: func(domain.Connection) (string, error) { return "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY", nil },
	}
	c := domain.Connection{AccountRef: "123456789012", CredentialRef: "awskey://AKIAIOSFODNN7EXAMPLE", SealedSecret: "sealed"}
	from := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	if _, err := p.GetUsage(context.Background(), domain.UsageRequest{Connection: c, From: from, To: from.AddDate(0, 0, 7)}); err != nil {
		t.Fatal(err)
	}
	if assumed || gotID != "AKIAIOSFODNN7EXAMPLE" || gotSecret == "" {
		t.Errorf("assumed=%v id=%q secret set=%v", assumed, gotID, gotSecret != "")
	}
	if err := p.Validate(context.Background(), c); err != nil {
		t.Errorf("validate: %v", err)
	}
	// a secret that cannot be opened says why and is not a customer action
	p.OpenSecret = func(domain.Connection) (string, error) {
		return "", errors.New("cipher: message authentication failed")
	}
	err := p.Validate(context.Background(), c)
	var pe domain.PublicError
	if !errors.As(err, &pe) || !strings.Contains(pe.Message, "NOTIFICATIONS_SECRET_KEY") || strings.Contains(pe.Message, "cipher") {
		t.Errorf("unopenable secret: %v", err)
	}
}
