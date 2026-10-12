package domain

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func approvedJob(t *testing.T, rec RecommendationInfo) Job {
	t.Helper()
	pr, err := BuildPlan(rec)
	if err != nil {
		t.Fatal(err)
	}
	by, at := "alice", time.Now()
	return Job{ID: "j1", Kind: pr.Kind, Status: Approved, Plan: pr.Plan, Risk: pr.Risk, RollbackPlan: pr.RollbackPlan, ApprovedBy: &by, ApprovedAt: &at}
}

func approvedRec() RecommendationInfo {
	r := resizeRec("m5.2xlarge", "m5.large")
	r.Status = "approved"
	return r
}

func failed(r DryRunReport) []string {
	var out []string
	for _, c := range r.Checks {
		if !c.OK {
			out = append(out, c.ID)
		}
	}
	return out
}

func TestDryRunReadyForAnApprovedCurrentPlan(t *testing.T) {
	rec := approvedRec()
	r := DryRun(DryRunInput{Job: approvedJob(t, rec), Rec: rec})
	if !r.Ready || len(failed(r)) != 0 || r.Executed {
		t.Fatalf("an approved, current plan must be ready and never executed: %+v", r)
	}
	if len(r.WouldRun) != 5 || !strings.Contains(r.WouldRun[0], "stop-instances") {
		t.Errorf("would_run: %v", r.WouldRun)
	}
}

func TestDryRunNotReadyWhenPlannedOnly(t *testing.T) {
	rec := approvedRec()
	j := approvedJob(t, rec)
	j.Status, j.ApprovedBy, j.ApprovedAt = Planned, nil, nil
	r := DryRun(DryRunInput{Job: j, Rec: rec})
	if r.Ready || strings.Join(failed(r), ",") != "approval" {
		t.Fatalf("a plan nobody approved cannot be ready, and only approval should fail: %v", failed(r))
	}
	if len(r.WouldRun) == 0 {
		t.Error("the commands are still shown so the plan can be reviewed")
	}
}

func TestDryRunCatchesDriftResidencyAndWithdrawnRecommendation(t *testing.T) {
	rec := approvedRec()
	j := approvedJob(t, rec)

	changed := rec
	changed.Details = map[string]string{"action": "modify", "resource_id": rec.Details["resource_id"], "current_type": "m5.2xlarge", "target_type": "m5.xlarge"}
	if r := DryRun(DryRunInput{Job: j, Rec: changed}); r.Ready || strings.Join(failed(r), ",") != "plan_current" {
		t.Errorf("a recommendation that changed after planning must fail plan_current: %v", failed(r))
	}
	if r := DryRun(DryRunInput{Job: j, Rec: rec, RecheckErr: errors.New("region not allowed")}); r.Ready || strings.Join(failed(r), ",") != "residency" {
		t.Errorf("residency: %v", failed(r))
	}
	applied := rec
	applied.Status = "applied"
	if r := DryRun(DryRunInput{Job: j, Rec: applied}); r.Ready || strings.Join(failed(r), ",") != "recommendation" {
		t.Errorf("an already applied recommendation: %v", failed(r))
	}
	if r := DryRun(DryRunInput{Job: j, RecErr: ErrNotFound}); r.Ready {
		t.Error("an unreadable recommendation cannot be ready")
	}
	done := j
	done.Status = Completed
	if r := DryRun(DryRunInput{Job: done, Rec: rec}); r.Ready || failed(r)[0] != "status" {
		t.Errorf("a finished job is not applied again: %v", failed(r))
	}
}

func TestDryRunRefusesAnUnsafeCommand(t *testing.T) {
	rec := approvedRec()
	j := approvedJob(t, rec)
	j.Plan.CLI = append([]string{"aws ec2 stop-instances --region eu-central-1 --instance-ids i-0abc123def4567890; rm -rf /"}, j.Plan.CLI...)
	r := DryRun(DryRunInput{Job: j, Rec: rec})
	if r.Ready || len(r.WouldRun) != 0 {
		t.Fatalf("a command with shell metacharacters must not be shown as runnable: %+v", r)
	}
}

func TestDryRunKeepsTheFinalTerminateHeld(t *testing.T) {
	rec := resizeRec("m5.large", "")
	rec.Details["action"] = "terminate"
	rec.Status = "approved"
	r := DryRun(DryRunInput{Job: approvedJob(t, rec), Rec: rec})
	if !r.Ready || len(r.WouldRun) != 2 || len(r.Held) != 1 || !strings.Contains(r.Held[0], "terminate-instances") {
		t.Fatalf("terminate must stay held back: run=%v held=%v failed=%v", r.WouldRun, r.Held, failed(r))
	}
	for _, c := range r.WouldRun {
		if strings.Contains(c, "terminate") {
			t.Errorf("terminate must never be in would_run: %s", c)
		}
	}
}
