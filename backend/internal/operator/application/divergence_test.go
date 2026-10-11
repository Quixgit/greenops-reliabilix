package application

import (
	"testing"
	"time"

	"github.com/quixgit/greenops-reliabilix/backend/internal/operator/domain"
)

func TestDivergencesShowWhereSourcesDisagree(t *testing.T) {
	now := time.Date(2026, 10, 11, 12, 0, 0, 0, time.UTC)
	pol := domain.AggregationPolicy{Priority: []string{"electricitymaps", "ons", "ember"}, MaxAgeHours: 72}
	rows := []domain.DivergenceRow{
		{Region: "sa-east-1", Provider: "electricitymaps", G: 300, N: 60, Newest: now},
		{Region: "sa-east-1", Provider: "ons", G: 90, N: 150, Newest: now.Add(-30 * time.Hour)},
		{Region: "sa-east-1", Provider: "ember", G: 105, N: 1, Newest: now.AddDate(0, -2, 0), Kind: "monthly"},
		{Region: "eu-central-1", Provider: "electricitymaps", G: 200, N: 40, Newest: now},
		{Region: "eu-central-1", Provider: "energy_charts", G: 210, N: 40, Newest: now}, // not in this policy: not compared
		{Region: "eu-central-1", Provider: "ember", G: 300, N: 1, Newest: now.AddDate(0, -1, 0), Kind: "monthly"},
		{Region: "lonely", Provider: "ons", G: 80, N: 20, Newest: now}, // one source, no anchor: left out
	}
	got := divergences(rows, pol, []string{"eu-central-1"}, now)
	if len(got) != 2 {
		t.Fatalf("regions: %+v", got)
	}
	sa := got[0]
	if sa.Region != "sa-east-1" || sa.Level != "disagrees" || sa.InUse || sa.Used != "electricitymaps" {
		t.Fatalf("sa-east-1: %+v", sa)
	}
	if sa.Ratio < 3.2 || sa.Ratio > 3.4 || sa.Anchor == nil || sa.Anchor.G != 105 {
		t.Fatalf("ratio/anchor: %+v", sa)
	}
	if sa.Sources[0].VsAnchorPct == nil || *sa.Sources[0].VsAnchorPct < 180 || *sa.Sources[0].VsAnchorPct > 190 {
		t.Fatalf("vs anchor: %+v", sa.Sources[0])
	}
	eu := got[1]
	// a lone hourly source (200) against the national figure (300): within 2x, so ok
	if eu.Region != "eu-central-1" || eu.Level != "ok" || !eu.InUse || len(eu.Sources) != 1 {
		t.Fatalf("eu-central-1: %+v", eu)
	}
}

func TestStaleSourceIsNotTheOneUsed(t *testing.T) {
	now := time.Date(2026, 10, 11, 12, 0, 0, 0, time.UTC)
	pol := domain.AggregationPolicy{Priority: []string{"electricitymaps", "ons"}, MaxAgeHours: 72}
	got := divergences([]domain.DivergenceRow{
		{Region: "r", Provider: "electricitymaps", G: 100, N: 40, Newest: now.AddDate(0, 0, -10)},
		{Region: "r", Provider: "ons", G: 110, N: 40, Newest: now},
	}, pol, nil, now)
	if len(got) != 1 || got[0].Used != "ons" || got[0].Level != "ok" {
		t.Fatalf("%+v", got)
	}
}

func TestShortHistoryOfALoneSourceIsNotCalledADisagreement(t *testing.T) {
	now := time.Now()
	pol := domain.AggregationPolicy{Priority: []string{"electricitymaps", "ember"}, MaxAgeHours: 72}
	got := divergences([]domain.DivergenceRow{
		{Region: "r", Provider: "electricitymaps", G: 200, N: 30, Newest: now},
		{Region: "r", Provider: "ember", G: 60, N: 1, Newest: now.AddDate(0, -1, 0), Kind: "monthly"},
	}, pol, nil, now)
	if len(got) != 1 || got[0].Level != "ok" {
		t.Fatalf("%+v", got)
	}
}
