package application

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/hibiken/asynq"

	"github.com/quixgit/greenops-reliabilix/backend/internal/cloudaccounts/domain"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/focus"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/queue"
)

type memRepo struct {
	exports map[string]*domain.ExportConfig
	conns   map[string]domain.Connection
	status  map[string]domain.SyncStatus
	through map[string]time.Time
	runs    []string
}

func newRepo(cs ...domain.Connection) *memRepo {
	r := &memRepo{conns: map[string]domain.Connection{}, status: map[string]domain.SyncStatus{}, through: map[string]time.Time{}}
	for _, c := range cs {
		r.conns[c.ID] = c
	}
	return r
}
func (r *memRepo) List(context.Context, string) ([]domain.Connection, error) { return nil, nil }
func (r *memRepo) Get(_ context.Context, _, id string) (domain.Connection, error) {
	c, ok := r.conns[id]
	if !ok {
		return domain.Connection{}, domain.ErrNotFound
	}
	return c, nil
}
func (r *memRepo) Create(_ context.Context, c domain.Connection) (domain.Connection, error) {
	c.ID = "c1"
	c.SyncStatus = domain.StatusPending
	r.conns[c.ID] = c
	return c, nil
}
func (r *memRepo) Delete(context.Context, string, string) error { return nil }
func (r *memRepo) SetExport(_ context.Context, _, id string, e *domain.ExportConfig) error {
	if r.exports == nil {
		r.exports = map[string]*domain.ExportConfig{}
	}
	r.exports[id] = e
	c := r.conns[id]
	c.Export = e
	r.conns[id] = c
	return nil
}
func (r *memRepo) SetStatus(_ context.Context, _, id string, s domain.SyncStatus, _ *string) error {
	r.status[id] = s
	return nil
}
func (r *memRepo) StartRun(context.Context, string, string) (string, error) { return "run1", nil }
func (r *memRepo) FinishRun(_ context.Context, _, _ string, ok bool, _ int, _ *string) error {
	if ok {
		r.runs = append(r.runs, "ok")
	} else {
		r.runs = append(r.runs, "failed")
	}
	return nil
}
func (r *memRepo) MarkSynced(_ context.Context, _, id string, through time.Time, _ int) error {
	r.through[id] = through
	return nil
}
func (r *memRepo) Audit(context.Context, string, string, string, map[string]any) error { return nil }
func (r *memRepo) ListRuns(context.Context, string, string) ([]domain.SyncRun, error) {
	return nil, nil
}
func (r *memRepo) ListForSync(context.Context) ([]domain.JobRef, error) {
	return []domain.JobRef{{TenantID: "t", ProjectID: "p", ConnectionID: "c1"}}, nil
}

type fakeProvider struct {
	validateErr, usageErr error
	gotReq                domain.UsageRequest
	findings              []domain.RightsizingFinding
	rightsizingErr        error
}

// GetRightsizing makes fakeProvider a domain.RightsizingProvider.
func (f *fakeProvider) GetRightsizing(context.Context, domain.Connection) ([]domain.RightsizingFinding, error) {
	return f.findings, f.rightsizingErr
}

type fakeSink struct {
	project  string
	findings []domain.RightsizingFinding
	calls    int
}

func (f *fakeSink) SubmitRightsizing(_ context.Context, _, project string, fs []domain.RightsizingFinding) error {
	f.calls, f.project, f.findings = f.calls+1, project, fs
	return nil
}

func (*fakeProvider) Provider() domain.ProviderType                       { return domain.AWS }
func (f *fakeProvider) Validate(context.Context, domain.Connection) error { return f.validateErr }
func (f *fakeProvider) GetUsage(_ context.Context, r domain.UsageRequest) ([]focus.RawBatch, error) {
	f.gotReq = r
	return []focus.RawBatch{{Provider: "aws", Source: "cost_explorer", Payload: []byte("{}")}}, f.usageErr
}

type fakeIngestor struct{ res focus.IngestResult }

func (f fakeIngestor) Ingest(context.Context, string, string, string, []focus.RawBatch) (focus.IngestResult, error) {
	return f.res, nil
}

type recorder struct {
	tasks    []string
	payloads []any
}

func (q *recorder) Enqueue(_ context.Context, typ string, p any, _ ...asynq.Option) error {
	q.tasks = append(q.tasks, typ)
	q.payloads = append(q.payloads, p)
	return nil
}

var day = func(s string) time.Time { v, _ := time.Parse(time.DateOnly, s); return v }

func newSvc(r *memRepo, p *fakeProvider, ing fakeIngestor, q *recorder) Service {
	return Service{Repo: r, Providers: domain.NewRegistry(p), Ingestor: ing, Queue: q,
		Log: slog.New(slog.NewTextHandler(io.Discard, nil)), PlatformAWSAccountID: "111122223333", BackfillDays: 30,
		Now: func() time.Time { return time.Date(2026, 10, 5, 15, 0, 0, 0, time.UTC) }}
}

func awsConn() domain.Connection {
	return domain.Connection{ID: "c1", TenantID: "t", ProjectID: "p", Provider: domain.AWS, AccountRef: "123456789012",
		CredentialRef: "arn:aws:iam::123456789012:role/ReliabilixR", ExternalID: "rlx-1"}
}

func TestConnectIssuesExternalIDAndTrustPolicy(t *testing.T) {
	s := newSvc(newRepo(), &fakeProvider{}, fakeIngestor{}, &recorder{})
	c, setup, err := s.Connect(context.Background(), domain.Connection{TenantID: "t", ProjectID: "p", Provider: domain.AWS,
		AccountRef: "123456789012", CredentialRef: "arn:aws:iam::123456789012:role/ReliabilixR"})
	if err != nil {
		t.Fatal(err)
	}
	if c.ExternalID == "" || setup.ExternalID != c.ExternalID || setup.TrustPolicy == nil {
		t.Fatalf("setup incomplete: %+v", setup)
	}
	if _, _, err := s.Connect(context.Background(), domain.Connection{ProjectID: "p", Provider: domain.Azure, AccountRef: "x", CredentialRef: "y"}); !errors.Is(err, domain.ErrUnsupportedProvider) {
		t.Errorf("azure must be unsupported in phase 1: %v", err)
	}
}

func TestVerify(t *testing.T) {
	repo, q := newRepo(awsConn()), &recorder{}
	s := newSvc(repo, &fakeProvider{}, fakeIngestor{}, q)
	if _, err := s.Verify(context.Background(), "t", "c1"); err != nil {
		t.Fatal(err)
	}
	if repo.status["c1"] != domain.StatusHealthy || len(q.tasks) != 1 || q.tasks[0] != queue.TaskSyncAWSAccount {
		t.Errorf("verify success: status=%v tasks=%v", repo.status["c1"], q.tasks)
	}
	repo2, q2 := newRepo(awsConn()), &recorder{}
	s2 := newSvc(repo2, &fakeProvider{validateErr: domain.ErrAccessDenied}, fakeIngestor{}, q2)
	if _, err := s2.Verify(context.Background(), "t", "c1"); !errors.Is(err, domain.ErrAccessDenied) {
		t.Fatalf("err = %v", err)
	}
	if repo2.status["c1"] != domain.StatusError || len(q2.tasks) != 0 {
		t.Errorf("verify failure must mark error and not sync: %v %v", repo2.status["c1"], q2.tasks)
	}
}

func TestRunSyncInitialBackfillThenChainsCarbon(t *testing.T) {
	repo, prov, q := newRepo(awsConn()), &fakeProvider{}, &recorder{}
	ing := fakeIngestor{res: focus.IngestResult{Records: 5, From: day("2026-09-10"), To: day("2026-10-04")}}
	if err := newSvc(repo, prov, ing, q).RunSync(context.Background(), "t", "c1"); err != nil {
		t.Fatal(err)
	}
	if got := prov.gotReq; !got.From.Equal(day("2026-09-05")) || !got.To.Equal(day("2026-10-05")) {
		t.Errorf("initial window = [%v, %v), want [2026-09-05, 2026-10-05)", got.From, got.To)
	}
	if !repo.through["c1"].Equal(day("2026-10-04")) {
		t.Errorf("synced_through = %v, want yesterday", repo.through["c1"])
	}
	if len(q.tasks) != 1 || q.tasks[0] != queue.TaskCalculateCarbon {
		t.Fatalf("chain broken: %v", q.tasks)
	}
	w := q.payloads[0].(queue.WindowPayload)
	if w.From != "2026-09-10" || w.To != "2026-10-05" || w.ProjectID != "p" {
		t.Errorf("carbon window = %+v", w)
	}
}

func TestRunSyncIncrementalOverlap(t *testing.T) {
	c := awsConn()
	th := day("2026-10-04")
	c.SyncedThrough = &th
	prov := &fakeProvider{}
	if err := newSvc(newRepo(c), prov, fakeIngestor{}, &recorder{}).RunSync(context.Background(), "t", "c1"); err != nil {
		t.Fatal(err)
	}
	if !prov.gotReq.From.Equal(day("2026-09-28")) { // synced_through - 6 days = 7-day overlap, inclusive
		t.Errorf("incremental from = %v", prov.gotReq.From)
	}
}

func TestRunSyncAccessDeniedMarksErrorAndIsNotRetryable(t *testing.T) {
	repo, q := newRepo(awsConn()), &recorder{}
	err := newSvc(repo, &fakeProvider{usageErr: domain.ErrAccessDenied}, fakeIngestor{}, q).RunSync(context.Background(), "t", "c1")
	if !errors.Is(err, domain.ErrAccessDenied) {
		t.Fatalf("err = %v", err)
	}
	if repo.status["c1"] != domain.StatusError || len(q.tasks) != 0 || repo.runs[0] != "failed" {
		t.Errorf("state after denial: %v %v %v", repo.status["c1"], q.tasks, repo.runs)
	}
}

func TestRunSyncDeletedConnectionIsNoop(t *testing.T) {
	if err := newSvc(newRepo(), &fakeProvider{}, fakeIngestor{}, &recorder{}).RunSync(context.Background(), "t", "gone"); err != nil {
		t.Errorf("deleted connection must not fail the job: %v", err)
	}
}

func TestFanOut(t *testing.T) {
	q := &recorder{}
	n, err := newSvc(newRepo(), &fakeProvider{}, fakeIngestor{}, q).FanOut(context.Background())
	if err != nil || n != 1 || q.tasks[0] != queue.TaskSyncAWSAccount {
		t.Errorf("fanout: %d %v %v", n, err, q.tasks)
	}
}

func TestRunRightsizingSubmitsFindingsToTheSink(t *testing.T) {
	sink := &fakeSink{}
	p := &fakeProvider{findings: []domain.RightsizingFinding{{ResourceID: "i-1", Region: "eu-central-1"}}}
	s := newSvc(newRepo(awsConn()), p, fakeIngestor{}, &recorder{})
	s.Rightsizing = sink
	if err := s.RunRightsizing(context.Background(), "t", "c1"); err != nil {
		t.Fatal(err)
	}
	if sink.calls != 1 || sink.project != "p" || len(sink.findings) != 1 {
		t.Fatalf("sink got %+v", sink)
	}
}

func TestRunRightsizingToleratesMissingPermissionAndConnection(t *testing.T) {
	sink := &fakeSink{}
	denied := &fakeProvider{rightsizingErr: domain.ErrAccessDenied}
	s := newSvc(newRepo(awsConn()), denied, fakeIngestor{}, &recorder{})
	s.Rightsizing = sink
	repo := s.Repo.(*memRepo)
	if err := s.RunRightsizing(context.Background(), "t", "c1"); err != nil {
		t.Fatalf("a missing optional permission must not fail the job: %v", err)
	}
	if repo.status["c1"] == domain.StatusError {
		t.Fatal("a missing rightsizing permission must not mark the connection unhealthy")
	}
	if err := s.RunRightsizing(context.Background(), "t", "gone"); err != nil {
		t.Fatalf("a deleted connection is a no-op: %v", err)
	}
	if sink.calls != 0 {
		t.Fatalf("nothing may be submitted, got %d calls", sink.calls)
	}
	boom := &fakeProvider{rightsizingErr: errors.New("throttled")}
	s2 := newSvc(newRepo(awsConn()), boom, fakeIngestor{}, &recorder{})
	s2.Rightsizing = sink
	if err := s2.RunRightsizing(context.Background(), "t", "c1"); err == nil {
		t.Fatal("transient errors must surface so the job is retried")
	}
}

func TestRunRightsizingWithoutSinkIsNoop(t *testing.T) {
	s := newSvc(newRepo(awsConn()), &fakeProvider{findings: []domain.RightsizingFinding{{}}}, fakeIngestor{}, &recorder{})
	if err := s.RunRightsizing(context.Background(), "t", "c1"); err != nil {
		t.Fatal(err)
	}
}

func TestSetExport(t *testing.T) {
	s := newSvc(newRepo(awsConn()), &fakeProvider{}, fakeIngestor{}, &recorder{})
	good := &domain.ExportConfig{Bucket: "acme-billing", Prefix: "exports", Name: "reliabilix", Region: "eu-central-1"}
	c, err := s.SetExport(context.Background(), "t", "c1", good)
	if err != nil || c.Export == nil || c.Export.Bucket != "acme-billing" {
		t.Fatalf("set export: %+v %v", c, err)
	}
	if _, err := s.SetExport(context.Background(), "t", "c1", &domain.ExportConfig{Bucket: "acme-billing", Prefix: "../x", Name: "n", Region: "eu-central-1"}); !errors.Is(err, domain.ErrInvalidConnection) {
		t.Fatalf("a traversal prefix must be rejected, got %v", err)
	}
	if c, err := s.SetExport(context.Background(), "t", "c1", nil); err != nil || c.Export != nil {
		t.Fatalf("clear export: %+v %v", c, err)
	}
	if _, err := s.SetExport(context.Background(), "t", "missing", good); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("unknown connection: %v", err)
	}
	azure := awsConn()
	azure.ID, azure.Provider = "c2", domain.Azure
	s2 := newSvc(newRepo(azure), &fakeProvider{}, fakeIngestor{}, &recorder{})
	if _, err := s2.SetExport(context.Background(), "t", "c2", good); !errors.Is(err, domain.ErrInvalidConnection) {
		t.Fatalf("exports are AWS only: %v", err)
	}
}

func TestGCPSetupGivesNonSecretInstructions(t *testing.T) {
	gcp := domain.Connection{ID: "g1", TenantID: "t", ProjectID: "p", Provider: domain.GCP, AccountRef: "acme-prod-123", ExternalID: "rlx-abc",
		CredentialRef: "bq://acme-billing-1/billing_export/gcp_billing_export_v1_AAAAAA_BBBBBB_CCCCCC"}
	s := newSvc(newRepo(gcp), &fakeProvider{}, fakeIngestor{}, &recorder{})
	s.PlatformGCPServiceAccount = "platform@reliabilix-prod.iam.gserviceaccount.com"
	st, err := s.SetupFor(context.Background(), "t", "g1")
	if err != nil {
		t.Fatal(err)
	}
	all := strings.Join(st.Steps, "\n")
	for _, want := range []string{"gcloud services enable bigquery.googleapis.com --project acme-billing-1", "Billing export",
		"bq update --set_label rlx-abc:1 acme-billing-1:billing_export", "platform@reliabilix-prod.iam.gserviceaccount.com", "no history"} {
		if !strings.Contains(all, want) {
			t.Errorf("steps lack %q:\n%s", want, all)
		}
	}
	if st.ExternalID != "rlx-abc" || st.PlatformAccountID != s.PlatformGCPServiceAccount || st.TrustPolicy != nil {
		t.Errorf("setup wrong: %+v", st)
	}
	s.PlatformGCPServiceAccount = ""
	if st, _ := s.SetupFor(context.Background(), "t", "g1"); !strings.Contains(strings.Join(st.Steps, "\n"), "Contact support") {
		t.Error("without a configured service account the customer must be told to ask for it")
	}
}

func TestSyncFailuresThatNeedTheCustomerAreNotRetried(t *testing.T) {
	for _, cause := range []error{domain.ErrBigQueryDisabled, domain.ErrOwnershipNotProven, domain.ErrExportNotFound, domain.ErrAccessDenied} {
		repo := newRepo(awsConn())
		p := &fakeProvider{usageErr: cause}
		s := newSvc(repo, p, fakeIngestor{}, &recorder{})
		if err := s.RunSync(context.Background(), "t", "c1"); !errors.Is(err, cause) {
			t.Fatalf("%v: %v", cause, err)
		}
		if repo.status["c1"] != domain.StatusError {
			t.Errorf("%v must mark the connection as errored until the customer fixes it", cause)
		}
		if msg := safeMessage(cause); msg == "" || strings.Contains(msg, "retried automatically") {
			t.Errorf("%v needs an actionable message, got %q", cause, msg)
		}
	}
}
