package domain

import (
	"errors"
	"strings"
	"testing"
)

func resizeRec(cur, target string) RecommendationInfo {
	return RecommendationInfo{Type: "rightsizing", CurrentRegion: "eu-central-1",
		Details: map[string]string{"action": "modify", "resource_id": "i-0abc123def4567890", "current_type": cur, "target_type": target}}
}

func TestResizePlanIsSpecificAndReversible(t *testing.T) {
	p, err := BuildPlan(resizeRec("m5.2xlarge", "m5.large"))
	if err != nil {
		t.Fatal(err)
	}
	if p.Kind != EC2Resize || p.Risk.Level != RiskMedium || p.RollbackPlan == "" || len(p.Plan.Steps) == 0 {
		t.Fatalf("plan incomplete: %+v", p)
	}
	joined := strings.Join(p.Plan.CLI, "\n")
	for _, want := range []string{"--region eu-central-1", "i-0abc123def4567890", `{"Value":"m5.large"}`, "stop-instances", "start-instances"} {
		if !strings.Contains(joined, want) {
			t.Errorf("CLI lacks %q:\n%s", want, joined)
		}
	}
	if !strings.Contains(p.RollbackPlan, "m5.2xlarge") || !strings.Contains(p.Plan.Terraform, `"m5.large"`) {
		t.Errorf("rollback must restore the original type, terraform must set the target: %+v", p)
	}
}

func TestRiskEscalatesForArchitectureAndLocalDisks(t *testing.T) {
	arm, _ := BuildPlan(resizeRec("m5.xlarge", "m6g.large"))
	if arm.Risk.Level != RiskHigh || !strings.Contains(strings.Join(arm.Risk.Factors, " "), "architecture") {
		t.Errorf("x86 -> Arm must be high risk: %+v", arm.Risk)
	}
	disk, _ := BuildPlan(resizeRec("m5d.xlarge", "m5d.large"))
	if disk.Risk.Level != RiskHigh || !strings.Contains(strings.Join(disk.Risk.Factors, " "), "instance-store") {
		t.Errorf("local disks must be high risk: %+v", disk.Risk)
	}
	same, _ := BuildPlan(resizeRec("m6g.xlarge", "m6g.large"))
	if same.Risk.Level != RiskMedium {
		t.Errorf("a plain resize is medium risk: %+v", same.Risk)
	}
	unknown, _ := BuildPlan(resizeRec("m5.xlarge", "weirdtype.large"))
	if unknown.Risk.Level != RiskHigh {
		t.Errorf("an unanalysable family must not be called low risk: %+v", unknown.Risk)
	}
}

func TestTerminateIsAlwaysHighRiskWithBackupFirst(t *testing.T) {
	r := resizeRec("m5.large", "")
	r.Details["action"] = "terminate"
	p, err := BuildPlan(r)
	if err != nil || p.Kind != EC2Terminate || p.Risk.Level != RiskHigh {
		t.Fatalf("%+v %v", p, err)
	}
	if !strings.Contains(p.Plan.CLI[0], "create-image") || !strings.Contains(p.Plan.CLI[len(p.Plan.CLI)-1], "# after the observation period") {
		t.Errorf("a backup must come first and the terminate command must stay commented out:\n%s", strings.Join(p.Plan.CLI, "\n"))
	}
}

func TestRegionShiftNeedsTwoDifferentValidRegions(t *testing.T) {
	ok := RecommendationInfo{Type: "region_shift", CurrentRegion: "us-east-1", RecommendedRegion: "eu-north-1"}
	p, err := BuildPlan(ok)
	if err != nil || p.Kind != RegionShift || p.Risk.Level != RiskHigh {
		t.Fatalf("%+v %v", p, err)
	}
	for _, bad := range []RecommendationInfo{
		{Type: "region_shift", CurrentRegion: "us-east-1", RecommendedRegion: "us-east-1"},
		{Type: "region_shift", CurrentRegion: "us-east-1", RecommendedRegion: "mars; id"},
		{Type: "region_shift", CurrentRegion: "us-east-1", RecommendedRegion: "$(id)"},
	} {
		if _, err := BuildPlan(bad); !errors.Is(err, ErrInvalidInput) {
			t.Errorf("%+v: want ErrInvalidInput, got %v", bad, err)
		}
	}
}

// Values end up inside shell commands, so anything that is not a plain identifier must be refused.
func TestInjectionAttemptsAreRejected(t *testing.T) {
	evil := []string{"i-0abc; rm -rf /", "i-0abc$(id)", "i-0abc`id`", "i-0abc && curl evil", "i-0abc\nrm", "", "i-ZZZ"}
	for _, id := range evil {
		r := resizeRec("m5.large", "m5.small")
		r.Details["resource_id"] = id
		if _, err := BuildPlan(r); !errors.Is(err, ErrInvalidInput) {
			t.Errorf("instance id %q must be rejected, got %v", id, err)
		}
	}
	for _, typ := range []string{`m5.large"}'; id #`, "m5 large", "m5.large;ls", "$(id).large", ""} {
		r := resizeRec("m5.large", typ)
		if _, err := BuildPlan(r); !errors.Is(err, ErrInvalidInput) {
			t.Errorf("target type %q must be rejected, got %v", typ, err)
		}
	}
	r := resizeRec("m5.large", "m5.small")
	r.CurrentRegion = "us-east-1; id"
	if _, err := BuildPlan(r); !errors.Is(err, ErrInvalidInput) {
		t.Errorf("region injection must be rejected, got %v", err)
	}
}

func TestUnsupportedTypesAndActions(t *testing.T) {
	if _, err := BuildPlan(RecommendationInfo{Type: "spot_migration"}); !errors.Is(err, ErrUnsupported) {
		t.Errorf("spot: %v", err)
	}
	if _, err := BuildPlan(RecommendationInfo{Type: "rightsizing", Details: map[string]string{"action": "explode"}}); !errors.Is(err, ErrInvalidInput) {
		t.Errorf("unknown action: %v", err)
	}
}

func TestTransitionsAndExecutionGuard(t *testing.T) {
	j := &Job{Status: Planned, RollbackPlan: "undo"}
	if err := j.Transition(Completed); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("planned -> completed must be impossible: %v", err)
	}
	if err := j.CanExecute(); !errors.Is(err, ErrNotApproved) {
		t.Fatalf("an unapproved job must not be executable: %v", err)
	}
	if err := j.Transition(Approved); err != nil {
		t.Fatal(err)
	}
	if err := j.CanExecute(); !errors.Is(err, ErrNotApproved) {
		t.Fatalf("approval needs an approver and a timestamp: %v", err)
	}
	by, now := "user-1", timeNow()
	j.ApprovedBy, j.ApprovedAt = &by, &now
	if err := j.CanExecute(); err != nil {
		t.Fatalf("approved job: %v", err)
	}
	for _, to := range []Status{Completed} {
		if err := j.Transition(to); err != nil {
			t.Fatal(err)
		}
	}
	if err := j.Transition(RolledBack); err != nil {
		t.Fatal(err)
	}
	if err := j.Transition(Approved); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("rolled_back is terminal: %v", err)
	}
	if st, ok := Outcome("completed").Status(); !ok || st != Completed {
		t.Error("outcome mapping")
	}
	if _, ok := Outcome("approved").Status(); ok {
		t.Error("an outcome must not be able to set arbitrary statuses")
	}
}
