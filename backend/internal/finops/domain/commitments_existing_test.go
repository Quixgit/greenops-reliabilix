package domain

import "testing"

func ptr(v float64) *float64 { return &v }

// A steady 100 a day, 70% target, 30% discount, no commitment yet: add 70%.
func steady(p CommitmentParams) ProviderCommitment {
	p.From = params().From
	p.To = params().To
	p.LookbackDays, p.TermYears, p.DiscountPct = 30, 1, map[string]float64{"aws": 30}
	if p.CoveragePct == 0 {
		p.CoveragePct = 70
	}
	return BuildCommitments(series("aws", p.From, repeat(100, 30)...), p).Providers[0]
}

func TestNothingKnownAboutExistingCommitmentsSaysSoAndKeepsTheFigures(t *testing.T) {
	pc := steady(CommitmentParams{})
	if pc.ExistingSource != "" || pc.Advice != AdviceBuy || !near(pc.AdditionalCoveragePct, 70) {
		t.Errorf("%+v", pc)
	}
	if !near(pc.MonthlySaving, 100*30.4*0.7*0.3) {
		t.Errorf("saving %v", pc.MonthlySaving)
	}
}

func TestOnlyWhatIsNotYetCoveredIsCounted(t *testing.T) {
	// committable == baseline here (flat 100 a day), so a 40% coverage is 40% of the baseline
	pc := steady(CommitmentParams{Read: []Existing{{Provider: "aws", Kind: "savings_plans", CoveragePct: ptr(40), UtilizationPct: ptr(97)}}})
	if pc.ExistingSource != "read" || pc.Advice != AdviceBuy || !near(pc.AdditionalCoveragePct, 30) {
		t.Fatalf("%+v", pc)
	}
	if !near(pc.CoveredMonthly, 100*30.4*0.3) || !near(pc.MonthlySaving, 100*30.4*0.3*0.3) {
		t.Errorf("covered %v saving %v: only the 30%% still to cover", pc.CoveredMonthly, pc.MonthlySaving)
	}
}

func TestTargetAlreadyCoveredMeansHold(t *testing.T) {
	pc := steady(CommitmentParams{Read: []Existing{{Provider: "aws", Kind: "savings_plans", CoveragePct: ptr(85), UtilizationPct: ptr(99)}}})
	if pc.Advice != AdviceHold || pc.MonthlySaving != 0 || pc.AdditionalCoveragePct != 0 {
		t.Errorf("%+v", pc)
	}
}

func TestAnUnderusedCommitmentIsFixedBeforeBuyingMore(t *testing.T) {
	pc := steady(CommitmentParams{Read: []Existing{{Provider: "aws", Kind: "savings_plans", CoveragePct: ptr(20), UtilizationPct: ptr(72), Unused: ptr(180)}}})
	if pc.Advice != AdviceReduce || pc.MonthlySaving != 0 || *pc.ExistingUnusedMonthly != 180 {
		t.Errorf("below 90%% use no purchase is advised and the waste is shown: %+v", pc)
	}
}

func TestWhatTheCustomerTypedWinsOverTheReading(t *testing.T) {
	pc := steady(CommitmentParams{
		Read:   []Existing{{Provider: "aws", Kind: "savings_plans", CoveragePct: ptr(10), UtilizationPct: ptr(99)}},
		Manual: map[string]ManualExisting{"aws": {CoveragePct: ptr(60), UtilizationPct: ptr(95)}},
	})
	if pc.ExistingSource != "entered" || !near(pc.AdditionalCoveragePct, 10) {
		t.Errorf("%+v", pc)
	}
}

func TestSeveralAccountsAreMergedByCommitmentSize(t *testing.T) {
	cov, util, _, ok := existingFor("aws", []Existing{
		{Provider: "aws", Kind: "savings_plans", CoveragePct: ptr(30), UtilizationPct: ptr(100), Commitment: ptr(900)},
		{Provider: "aws", Kind: "savings_plans", CoveragePct: ptr(50), UtilizationPct: ptr(50), Commitment: ptr(100)},
		{Provider: "aws", Kind: "reservations", CoveragePct: ptr(99), UtilizationPct: ptr(99)}, // plans are preferred
	})
	if !ok || *cov != 40 || *util != 95 {
		t.Errorf("coverage %v utilization %v", *cov, *util)
	}
}
