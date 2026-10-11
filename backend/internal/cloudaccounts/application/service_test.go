package application

import (
	"context"
	"encoding/json"
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
	quality *domain.Quality // quality stored by the latest successful sync
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
func (r *memRepo) FinishRun(_ context.Context, _, _ string, ok bool, _ int, _ *string, _ *domain.Quality) error {
	if ok {
		r.runs = append(r.runs, "ok")
	} else {
		r.runs = append(r.runs, "failed")
	}
	return nil
}
func (r *memRepo) MarkSynced(_ context.Context, _, id string, through time.Time, _ int, q *domain.Quality) error {
	r.through[id] = through
	r.quality = q
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
	if _, _, err := s.Connect(context.Background(), domain.Connection{ProjectID: "p", Provider: domain.Azure, AccountRef: azureSub,
		CredentialRef: "azblob://" + azureTenant + "@acmebilling/exports/rlx/daily"}); !errors.Is(err, domain.ErrUnsupportedProvider) {
		t.Errorf("a provider that is not registered must be refused: %v", err)
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

const (
	azureSub    = "11111111-2222-3333-4444-555555555555"
	azureTenant = "99999999-8888-7777-6666-555555555555"
)

func TestAzureSetupGivesNonSecretInstructions(t *testing.T) {
	az := domain.Connection{ID: "z1", TenantID: "t", ProjectID: "p", Provider: domain.Azure, AccountRef: azureSub, ExternalID: "rlx-abc",
		CredentialRef: "azblob://" + azureTenant + "@acmebilling/exports/rlx/daily"}
	s := newSvc(newRepo(az), &fakeProvider{}, fakeIngestor{}, &recorder{})
	s.PlatformAzureAppID = "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"
	st, err := s.SetupFor(context.Background(), "t", "z1")
	if err != nil {
		t.Fatal(err)
	}
	all := strings.Join(st.Steps, "\n")
	for _, want := range []string{"Cost and usage details (FOCUS)", "storage account acmebilling, container exports, directory rlx/daily",
		"--metadata rlx_abc=1", "az ad sp create --id aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee", "Storage Blob Data Reader"} {
		if !strings.Contains(all, want) {
			t.Errorf("steps lack %q:\n%s", want, all)
		}
	}
	if st.ExternalID != "rlx-abc" || st.PlatformAccountID != s.PlatformAzureAppID || st.TrustPolicy != nil {
		t.Errorf("setup wrong: %+v", st)
	}
	s.PlatformAzureAppID = ""
	if st, _ := s.SetupFor(context.Background(), "t", "z1"); !strings.Contains(strings.Join(st.Steps, "\n"), "Contact support") {
		t.Error("without a configured application the customer must be told to ask for it")
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

// ---- cost data quality (ADR-0024) ----

type controlProvider struct {
	*fakeProvider
	control []focus.ControlTotal
	err     error
}

func (c controlProvider) GetControlTotals(context.Context, domain.UsageRequest) ([]focus.ControlTotal, error) {
	return c.control, c.err
}

func syncWithControl(t *testing.T, cp controlProvider, stored []focus.ControlTotal, synced *time.Time) (*memRepo, *recorder) {
	t.Helper()
	conn := awsConn()
	conn.SyncedThrough = synced
	repo := newRepo(conn)
	q := &recorder{}
	s := Service{Repo: repo, Providers: domain.NewRegistry(cp), Ingestor: fakeIngestor{res: focus.IngestResult{Records: 3, From: day("2026-09-20"), To: day("2026-10-04"), Totals: stored}}, Queue: q,
		Log: slog.New(slog.NewTextHandler(io.Discard, nil)), BackfillDays: 30, Now: func() time.Time { return time.Date(2026, 10, 5, 15, 0, 0, 0, time.UTC) }}
	if err := s.RunSync(context.Background(), "t", "c1"); err != nil {
		t.Fatal(err)
	}
	return repo, q
}

func TestSyncStoresAnOKQualityAndMovesTheCursorToYesterday(t *testing.T) {
	cp := controlProvider{fakeProvider: &fakeProvider{}, control: []focus.ControlTotal{{Day: "2026-10-03", Currency: "USD", Billed: 50}}}
	repo, q := syncWithControl(t, cp, []focus.ControlTotal{{Day: "2026-10-03", Currency: "USD", Billed: 50}}, nil)
	if repo.quality == nil || repo.quality.Status != domain.QualityOK {
		t.Fatalf("quality = %+v", repo.quality)
	}
	if !repo.through["c1"].Equal(day("2026-10-04")) {
		t.Errorf("cursor = %v, want yesterday", repo.through["c1"])
	}
	if len(q.tasks) != 1 {
		t.Errorf("tasks = %v", q.tasks)
	}
}

// A day that does not add up must not be left behind the cursor: the next sync has to read it again.
func TestSyncRewindsTheCursorToTheEarliestMismatchedDay(t *testing.T) {
	cp := controlProvider{fakeProvider: &fakeProvider{}, control: []focus.ControlTotal{
		{Day: "2026-09-28", Currency: "USD", Billed: 100}, {Day: "2026-10-01", Currency: "USD", Billed: 100}}}
	repo, _ := syncWithControl(t, cp, []focus.ControlTotal{{Day: "2026-09-28", Currency: "USD", Billed: 40}, {Day: "2026-10-01", Currency: "USD", Billed: 100}}, nil)
	if repo.quality.Status != domain.QualityMismatch || repo.quality.MismatchDays != 1 {
		t.Fatalf("quality = %+v", repo.quality)
	}
	if !repo.through["c1"].Equal(day("2026-09-27")) {
		t.Errorf("cursor = %v, want the day before the first mismatch (2026-09-27)", repo.through["c1"])
	}
}

func TestRewindIsBoundedSoAnUnfixableDayCannotCauseEverLargerReads(t *testing.T) {
	cp := controlProvider{fakeProvider: &fakeProvider{}, control: []focus.ControlTotal{{Day: "2026-09-05", Currency: "USD", Billed: 100}}}
	repo, _ := syncWithControl(t, cp, nil, nil) // mismatch a month back
	want := day("2026-10-04").AddDate(0, 0, -maxRewindDays)
	if !repo.through["c1"].Equal(want) {
		t.Errorf("cursor = %v, want %v (at most %d days back)", repo.through["c1"], want, maxRewindDays)
	}
}

func TestUnverifiableSyncIsStoredAsUncheckedNotFailed(t *testing.T) {
	cp := controlProvider{fakeProvider: &fakeProvider{}, err: errors.New("throttled")}
	repo, _ := syncWithControl(t, cp, nil, nil)
	if repo.quality == nil || repo.quality.Status != domain.QualityUnchecked || len(repo.runs) != 1 || repo.runs[0] != "ok" {
		t.Errorf("a failing control query must not fail the sync: quality=%+v runs=%v", repo.quality, repo.runs)
	}
	plain, _ := syncWithControl(t, controlProvider{fakeProvider: &fakeProvider{}}, nil, nil) // control == nil: export-style connection
	if plain.quality.Status != domain.QualityUnchecked {
		t.Errorf("quality = %+v", plain.quality)
	}
}

type failingQueue struct{ recorder }

func (*failingQueue) Enqueue(context.Context, string, any, ...asynq.Option) error {
	return errors.New("redis down")
}

func TestLostCarbonEnqueueDoesNotFailAnAlreadySuccessfulSync(t *testing.T) {
	repo := newRepo(awsConn())
	s := Service{Repo: repo, Providers: domain.NewRegistry(&fakeProvider{}), Ingestor: fakeIngestor{res: focus.IngestResult{Records: 2, From: day("2026-10-01"), To: day("2026-10-02")}},
		Queue: &failingQueue{}, Log: slog.New(slog.NewTextHandler(io.Discard, nil)), BackfillDays: 30, Now: func() time.Time { return time.Date(2026, 10, 5, 15, 0, 0, 0, time.UTC) }}
	if err := s.RunSync(context.Background(), "t", "c1"); err != nil {
		t.Fatalf("the data is stored; failing the job would re-read and re-bill it: %v", err)
	}
	if len(repo.runs) != 1 || repo.runs[0] != "ok" {
		t.Errorf("runs = %v", repo.runs)
	}
}

type fakeSealer struct{ on bool }

func (f fakeSealer) Configured() bool                       { return f.on }
func (f fakeSealer) Seal(plain, aad string) (string, error) { return "sealed(" + aad + ")", nil }

func TestAzureServicePrincipalSecretIsSealedAndNeverReturned(t *testing.T) {
	const secret = "test-only-azure-client-secret-not-a-real-credential"
	repo := newRepo()
	s := newSvc(repo, &fakeProvider{}, fakeIngestor{}, &recorder{})
	s.Providers = domain.NewRegistry(&fakeProvider{}, azureStub{})
	s.Secrets = fakeSealer{on: true}
	in := domain.Connection{TenantID: "t", ProjectID: "p", Provider: domain.Azure, AccountRef: azureSub, ClientSecret: secret,
		CredentialRef: "azcost://" + azureTenant + "@b8b97d8b-fec7-4b74-a232-b0b605ff0959"}
	c, setup, err := s.Connect(context.Background(), in)
	if err != nil {
		t.Fatal(err)
	}
	stored := repo.conns["c1"]
	if stored.ClientSecret != "" || !strings.HasPrefix(stored.SealedSecret, "sealed(t:rlx-") || strings.Contains(stored.SealedSecret, secret) {
		t.Errorf("the secret must be stored sealed and bound to the organization and connection token: %+v", stored)
	}
	out, _ := json.Marshal(map[string]any{"connection": c, "setup": setup})
	if strings.Contains(string(out), secret) || strings.Contains(string(out), "sealed(") {
		t.Errorf("the response must carry neither the secret nor its sealed form: %s", out)
	}
	if !strings.Contains(strings.Join(setup.Steps, "\n"), "Cost Management Reader") {
		t.Errorf("steps: %v", setup.Steps)
	}

	// refused: no secret, a secret on another kind, and no sealing key
	in.ClientSecret = ""
	if _, _, err := s.Connect(context.Background(), in); !errors.Is(err, domain.ErrInvalidConnection) {
		t.Errorf("a service principal without a secret: %v", err)
	}
	aws := awsConn()
	aws.ClientSecret, aws.ID, aws.ExternalID = secret, "", ""
	if _, _, err := s.Connect(context.Background(), aws); !errors.Is(err, domain.ErrInvalidConnection) {
		t.Errorf("a secret on an AWS connection: %v", err)
	}
	in.ClientSecret = secret
	s.Secrets = fakeSealer{on: false}
	if _, _, err := s.Connect(context.Background(), in); !errors.Is(err, ErrSecretsUnavailable) {
		t.Errorf("no sealing key: %v", err)
	}
}

// azureStub is a registered Azure provider (the real one lives in providers/azure).
type azureStub struct{}

func (azureStub) Provider() domain.ProviderType                     { return domain.Azure }
func (azureStub) Validate(context.Context, domain.Connection) error { return nil }
func (azureStub) GetUsage(context.Context, domain.UsageRequest) ([]focus.RawBatch, error) {
	return nil, nil
}

func TestAWSAccessKeyAndGoogleKeyAreSealedAndNeverReturned(t *testing.T) {
	const secretKey = "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY"
	const saJSON = `{"type":"service_account","project_id":"acme-prod-123","private_key":"-----BEGIN PRIVATE KEY-----\nMIIE\n-----END PRIVATE KEY-----\n","client_email":"reliabilix@acme-prod-123.iam.gserviceaccount.com"}`
	repo := newRepo()
	s := newSvc(repo, &fakeProvider{}, fakeIngestor{}, &recorder{})
	s.Secrets = fakeSealer{on: true}
	aws := domain.Connection{TenantID: "t", ProjectID: "p", Provider: domain.AWS, AccountRef: "123456789012",
		CredentialRef: "awskey://AKIAIOSFODNN7EXAMPLE", ClientSecret: secretKey}
	c, setup, err := s.Connect(context.Background(), aws)
	if err != nil {
		t.Fatal(err)
	}
	if stored := repo.conns["c1"]; stored.ClientSecret != "" || !strings.HasPrefix(stored.SealedSecret, "sealed(t:rlx-") {
		t.Errorf("aws key must be stored sealed: %+v", stored)
	}
	out, _ := json.Marshal(map[string]any{"connection": c, "setup": setup})
	if strings.Contains(string(out), secretKey) || strings.Contains(string(out), "sealed(") || !strings.Contains(string(out), `"access":"`) {
		t.Errorf("response: %s", out)
	}
	if !strings.Contains(strings.Join(setup.Steps, "\n"), "Cost Explorer") || setup.TrustPolicy != nil {
		t.Errorf("setup of the access-key mode: %+v", setup)
	}
	// without the secret, or with a wrong shape, the key mode is refused
	aws.ClientSecret = ""
	if _, _, err := s.Connect(context.Background(), aws); !errors.Is(err, domain.ErrInvalidConnection) {
		t.Errorf("no secret: %v", err)
	}

	s.Providers = domain.NewRegistry(&fakeProvider{}, gcpStub{})
	g := domain.Connection{TenantID: "t", ProjectID: "p", Provider: domain.GCP, AccountRef: "acme-prod-123",
		CredentialRef: "bq://acme-billing-1/billing_export/gcp_billing_export_v1_AAAAAA_BBBBBB_CCCCCC", ClientSecret: saJSON}
	_, gsetup, err := s.Connect(context.Background(), g)
	if err != nil {
		t.Fatal(err)
	}
	if stored := repo.conns["c1"]; stored.SealedSecret == "" || stored.ClientSecret != "" {
		t.Errorf("gcp key must be stored sealed: %+v", stored)
	}
	if !strings.Contains(strings.Join(gsetup.Steps, "\n"), "BigQuery Job User") {
		t.Errorf("steps: %v", gsetup.Steps)
	}
	g.ClientSecret = `{"type":"authorized_user"}`
	if _, _, err := s.Connect(context.Background(), g); !errors.Is(err, domain.ErrInvalidConnection) {
		t.Errorf("a user credential file must be refused: %v", err)
	}
	// the platform-shared GCP mode takes no secret
	g.ClientSecret = ""
	if _, gs, err := s.Connect(context.Background(), g); err != nil || !strings.Contains(strings.Join(gs.Steps, "\n"), "label") {
		t.Errorf("shared dataset mode: %v %v", err, gs.Steps)
	}
}

type gcpStub struct{ azureStub }

func (gcpStub) Provider() domain.ProviderType { return domain.GCP }
