package application

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/quixgit/greenops-reliabilix/backend/internal/recommendations/domain"
)

type priceRepo struct {
	domain.Repository
	saved map[string]float64
	src   []string
}

func (r *priceRepo) SavePriceIndex(_ context.Context, provider, region string, idx float64, source string) error {
	r.saved[provider+"/"+region] = idx
	r.src = append(r.src, source)
	return nil
}

type stubSource struct {
	idx  []PriceEntry
	errs map[string]error
}

func (stubSource) Provider() string { return "aws" }
func (stubSource) Basis() string    { return "aws_ec2_ondemand_basket" }

func (s stubSource) BuildIndex(context.Context, []string) ([]PriceEntry, map[string]error, error) {
	return s.idx, s.errs, nil
}

func TestRefreshPriceIndexStoresWhatItGotAndReportsWhatFailed(t *testing.T) {
	repo := &priceRepo{saved: map[string]float64{}}
	svc := Service{Repo: repo, Log: slog.New(slog.NewTextHandler(io.Discard, nil)), Now: func() time.Time { return time.Date(2026, 10, 12, 3, 20, 0, 0, time.UTC) }}
	n, err := svc.RefreshPriceIndex(context.Background(), stubSource{
		idx:  []PriceEntry{{Region: "us-east-1", Idx: 1, N: 9}, {Region: "eu-central-1", Idx: 1.1756, N: 9}},
		errs: map[string]error{"sa-east-1": errors.New("page missing")},
	}, nil)
	if n != 2 || repo.saved["aws/eu-central-1"] != 1.1756 || repo.src[0] != "aws_ec2_ondemand_basket:2026-10-12" {
		t.Fatalf("stored %d %+v %v", n, repo.saved, repo.src)
	}
	if err == nil {
		t.Fatal("a failed region must be reported so the job is seen to be incomplete")
	}
}
