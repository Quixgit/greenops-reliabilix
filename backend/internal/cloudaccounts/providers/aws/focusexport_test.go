package aws

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"

	"github.com/quixgit/greenops-reliabilix/backend/internal/cloudaccounts/domain"
)

// fakeS3 serves objects from a map; a missing key behaves like S3 (NoSuchKey).
type fakeS3 struct {
	objects map[string][]byte
	gets    []string
	err     error
}

func (f *fakeS3) GetObject(_ context.Context, in *s3.GetObjectInput, _ ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
	f.gets = append(f.gets, *in.Key)
	if f.err != nil {
		return nil, f.err
	}
	b, ok := f.objects[*in.Key]
	if !ok {
		return nil, &s3types.NoSuchKey{}
	}
	return &s3.GetObjectOutput{Body: io.NopCloser(bytes.NewReader(b))}, nil
}

func gz(t *testing.T, s string) []byte {
	t.Helper()
	var b bytes.Buffer
	w := gzip.NewWriter(&b)
	if _, err := w.Write([]byte(s)); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

const exportHeader = "BilledCost,EffectiveCost,BillingCurrency,ChargePeriodStart,RegionId,ServiceName,ServiceCategory,ConsumedQuantity,ConsumedUnit,x_UsageType,x_ServiceCode\n"

func exportConn() domain.Connection {
	c := conn()
	c.Export = &domain.ExportConfig{Bucket: "acme-billing", Prefix: "exports", Name: "rlx", Region: "eu-central-1"}
	return c
}

func s3Provider(f *fakeS3) *Provider {
	return &Provider{AssumeS3: func(_ context.Context, _, _, _ string) (S3API, error) { return f, nil }}
}

func manifest(t *testing.T, month string, files ...string) []byte {
	t.Helper()
	b, err := json.Marshal(map[string]any{"dataFiles": files})
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestExportBatchAggregatesIntoDailyCostAndMeasuredRows(t *testing.T) {
	csv := exportHeader +
		// two hourly EC2 rows of the same day collapse into one; BoxUsage is running time
		"1.00,0.90,USD,2026-09-01T00:00:00Z,eu-central-1,Amazon Elastic Compute Cloud,Compute,1,Hrs,EUC1-BoxUsage:m5.large,AmazonEC2\n" +
		"1.00,0.90,USD,2026-09-01T01:00:00Z,eu-central-1,Amazon Elastic Compute Cloud,Compute,1,Hrs,EUC1-BoxUsage:m5.large,AmazonEC2\n" +
		// an EBS line of the same service is "EC2 - Other": it must not be merged into the compute line
		"0.50,0.50,USD,2026-09-01T00:00:00Z,eu-central-1,Amazon Elastic Compute Cloud,Storage,10,GB-Mo,EUC1-EBS:VolumeUsage.gp3,AmazonEC2\n" +
		"0.20,0.20,USD,2026-09-01T00:00:00Z,eu-central-1,Amazon Simple Storage Service,Storage,3.5,GB-Mo,EUC1-TimedStorage-ByteHrs,AmazonS3\n" +
		// outside the requested window
		"9.00,9.00,USD,2026-08-31T00:00:00Z,eu-central-1,Amazon Simple Storage Service,Storage,1,GB-Mo,EUC1-TimedStorage-ByteHrs,AmazonS3\n"
	f := &fakeS3{objects: map[string][]byte{
		"exports/rlx/metadata/BILLING_PERIOD=2026-09/rlx-Manifest.json": manifest(t, "2026-09", "exports/rlx/data/BILLING_PERIOD=2026-09/rlx-00001.csv.gz"),
		"exports/rlx/data/BILLING_PERIOD=2026-09/rlx-00001.csv.gz":      gz(t, csv),
	}}
	from := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	batches, err := s3Provider(f).GetUsage(context.Background(), domain.UsageRequest{Connection: exportConn(), From: from, To: from.AddDate(0, 0, 7)})
	if err != nil || len(batches) != 1 || batches[0].Source != SourceFocusExport {
		t.Fatalf("batches=%v err=%v", batches, err)
	}
	var out struct {
		Cost []struct {
			Service  string  `json:"service"`
			Category string  `json:"category"`
			Billed   float64 `json:"billed"`
		} `json:"cost"`
		EC2Hours []struct {
			InstanceType string  `json:"instance_type"`
			Hours        float64 `json:"hours"`
		} `json:"ec2_hours"`
		S3 []struct {
			GBMonths float64 `json:"gb_months"`
		} `json:"s3_gb_months"`
	}
	if err := json.Unmarshal(batches[0].Payload, &out); err != nil {
		t.Fatal(err)
	}
	cost := map[string]float64{}
	for _, c := range out.Cost {
		cost[c.Service] += c.Billed
	}
	if cost[ec2ComputeService] != 2 || cost[ec2OtherService] != 0.5 || cost["Amazon Simple Storage Service"] != 0.2 || len(out.Cost) != 3 {
		t.Errorf("cost aggregation wrong (EC2 compute vs other, window filter): %+v", out.Cost)
	}
	if len(out.EC2Hours) != 1 || out.EC2Hours[0].InstanceType != "m5.large" || out.EC2Hours[0].Hours != 2 {
		t.Errorf("instance hours wrong: %+v", out.EC2Hours)
	}
	if len(out.S3) != 1 || out.S3[0].GBMonths != 3.5 {
		t.Errorf("s3 storage wrong: %+v", out.S3)
	}
	for _, k := range f.gets {
		if !strings.HasPrefix(k, "exports/rlx/") {
			t.Errorf("read outside the export folder: %s", k)
		}
	}
}

func TestExportMissingCurrentMonthIsSkippedButNothingAtAllIsAnError(t *testing.T) {
	from := time.Date(2026, 8, 25, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 9, 3, 0, 0, 0, 0, time.UTC)
	f := &fakeS3{objects: map[string][]byte{
		"exports/rlx/metadata/BILLING_PERIOD=2026-08/rlx-Manifest.json": manifest(t, "2026-08", "exports/rlx/data/BILLING_PERIOD=2026-08/rlx-00001.csv.gz"),
		"exports/rlx/data/BILLING_PERIOD=2026-08/rlx-00001.csv.gz":      gz(t, exportHeader+"1,1,USD,2026-08-26T00:00:00Z,us-east-1,AWS Lambda,Compute,,,,AWSLambda\n"),
	}}
	if _, err := s3Provider(f).GetUsage(context.Background(), domain.UsageRequest{Connection: exportConn(), From: from, To: to}); err != nil {
		t.Fatalf("a month that is not delivered yet must be skipped: %v", err)
	}
	empty := &fakeS3{objects: map[string][]byte{}}
	if _, err := s3Provider(empty).GetUsage(context.Background(), domain.UsageRequest{Connection: exportConn(), From: from, To: to}); !errors.Is(err, domain.ErrExportNotFound) {
		t.Fatalf("no manifest at all must surface as ErrExportNotFound, got %v", err)
	}
}

func TestManifestCannotPointOutsideTheExportFolder(t *testing.T) {
	for _, evil := range []string{"other-folder/secret.csv.gz", "exports/rlx/data/../../secret.csv.gz", "exports/rlx/metadata/x.csv.gz"} {
		f := &fakeS3{objects: map[string][]byte{
			"exports/rlx/metadata/BILLING_PERIOD=2026-09/rlx-Manifest.json": manifest(t, "2026-09", evil),
			evil: gz(t, exportHeader),
		}}
		from := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
		_, err := s3Provider(f).GetUsage(context.Background(), domain.UsageRequest{Connection: exportConn(), From: from, To: from.AddDate(0, 0, 1)})
		if err == nil {
			t.Errorf("manifest entry %q must be rejected", evil)
		}
		for _, k := range f.gets {
			if k == evil {
				t.Errorf("the platform must never fetch %q", evil)
			}
		}
	}
}

func TestExportRejectsBadFiles(t *testing.T) {
	from := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	cases := map[string][]byte{
		"not gzip":          []byte("plain text"),
		"missing column":    gz(t, "BilledCost,ServiceName\n1,x\n"),
		"short row":         gz(t, exportHeader+"1,1,USD\n"),
		"bad charge period": gz(t, exportHeader+"1,1,USD,not-a-date,us-east-1,S,Compute,1,Hrs,u,c\n"),
	}
	for name, body := range cases {
		f := &fakeS3{objects: map[string][]byte{
			"exports/rlx/metadata/BILLING_PERIOD=2026-09/rlx-Manifest.json": manifest(t, "2026-09", "exports/rlx/data/BILLING_PERIOD=2026-09/a.csv.gz"),
			"exports/rlx/data/BILLING_PERIOD=2026-09/a.csv.gz":              body,
		}}
		if _, err := s3Provider(f).GetUsage(context.Background(), domain.UsageRequest{Connection: exportConn(), From: from, To: from.AddDate(0, 0, 1)}); err == nil {
			t.Errorf("%s: want an error", name)
		}
	}
}

func TestExportAccessDeniedAndValidate(t *testing.T) {
	denied := &fakeS3{err: apiErr{"AccessDenied"}}
	if err := s3Provider(denied).Validate(context.Background(), exportConn()); !errors.Is(err, domain.ErrAccessDenied) {
		t.Fatalf("Validate must classify S3 AccessDenied: %v", err)
	}
	if err := s3Provider(&fakeS3{objects: map[string][]byte{}}).Validate(context.Background(), exportConn()); !errors.Is(err, domain.ErrExportNotFound) {
		t.Fatalf("Validate with no manifest: %v", err)
	}
	now := time.Now().UTC()
	key := "exports/rlx/metadata/BILLING_PERIOD=" + now.Format("2006-01") + "/rlx-Manifest.json"
	ok := &fakeS3{objects: map[string][]byte{key: manifest(t, "", "exports/rlx/data/x.csv.gz")}}
	if err := s3Provider(ok).Validate(context.Background(), exportConn()); err != nil {
		t.Fatalf("Validate with a manifest: %v", err)
	}
}

func TestExportConnectionNeverCallsCostExplorer(t *testing.T) {
	ce := &fakeCE{}
	p := &Provider{
		Assume: func(context.Context, string, string) (CEAPI, error) { return ce, nil },
		AssumeS3: func(context.Context, string, string, string) (S3API, error) {
			return &fakeS3{objects: map[string][]byte{}}, nil
		},
	}
	from := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	_, _ = p.GetUsage(context.Background(), domain.UsageRequest{Connection: exportConn(), From: from, To: from.AddDate(0, 0, 1)})
	if len(ce.calls) != 0 {
		t.Fatalf("an export connection must not issue Cost Explorer cost queries (duplicate cost lines), got %d", len(ce.calls))
	}
}
