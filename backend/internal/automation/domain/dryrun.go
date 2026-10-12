package domain

import (
	"errors"
	"fmt"
	"reflect"
	"regexp"
)

// A command that ends up in a person's shell must have the shape the planner produces: one `aws ec2 <verb>` with
// long options whose values are plain tokens or a single-quoted JSON value without quotes inside.
var commandRe = regexp.MustCompile(`^aws ec2 [a-z-]+(?: [a-z][a-z-]*)?(?: --[a-z-]+(?: (?:[A-Za-z0-9._-]+|"[A-Za-z0-9._-]+"|'\{"Value":"[A-Za-z0-9._-]+"\}'))?)*$`)

// A line the planner leaves as a comment (for example the final terminate, after an observation period) is shown
// but is never counted as something the dry run would run.
var heldRe = regexp.MustCompile(`^# [a-z ]+: (aws ec2 .+)$`)

// DryRunCheck is one thing the dry run verified.
type DryRunCheck struct {
	ID     string `json:"id"`
	OK     bool   `json:"ok"`
	Detail string `json:"detail,omitempty"`
}

// DryRunReport is what executing a plan would do, decided without doing it. It never changes a job, a
// recommendation or a customer's cloud (ADR-0048).
type DryRunReport struct {
	JobID string `json:"job_id"`
	// Ready is true when every check passed and the plan is approved: applying it now would be allowed.
	Ready    bool          `json:"ready"`
	Checks   []DryRunCheck `json:"checks"`
	WouldRun []string      `json:"would_run"`
	// Held are commands the plan keeps commented out on purpose; a person enables them later.
	Held  []string `json:"held"`
	Steps []string `json:"steps"`
	// Executed is always false: the platform applies nothing (ADR-0017); the field says so to a pipeline reading it.
	Executed bool `json:"executed"`
}

// DryRunInput is the current state the job is checked against.
type DryRunInput struct {
	Job Job
	// Rec is the recommendation as it is now; RecErr is why it could not be read.
	Rec    RecommendationInfo
	RecErr error
	// RecheckErr is the project's data-residency policy verdict now (nil = allowed).
	RecheckErr error
}

// DryRun checks an approved plan against the present: still approved by a person, rollback plan present, the
// recommendation not withdrawn or applied elsewhere, data residency still allows it, the plan still equals what
// the planner would build today, and every command has the safe shape. It performs no I/O.
func DryRun(in DryRunInput) DryRunReport {
	j := in.Job
	rep := DryRunReport{JobID: j.ID, WouldRun: []string{}, Held: []string{}, Steps: []string{}}
	add := func(id string, ok bool, detail string) {
		rep.Checks = append(rep.Checks, DryRunCheck{ID: id, OK: ok, Detail: detail})
	}

	live := j.Status == Planned || j.Status == Approved
	if live {
		add("status", true, string(j.Status))
	} else {
		add("status", false, fmt.Sprintf("the job is %s: a finished or withdrawn plan is not applied again", j.Status))
	}
	if err := j.CanExecute(); err != nil {
		add("approval", false, err.Error())
	} else {
		add("approval", true, "")
	}
	add("rollback", j.RollbackPlan != "", "")

	switch {
	case in.RecErr != nil:
		add("recommendation", false, in.RecErr.Error())
	case in.Rec.Status != "approved":
		add("recommendation", false, fmt.Sprintf("the recommendation is %s, not approved", in.Rec.Status))
	default:
		add("recommendation", true, "")
		if in.RecheckErr != nil {
			add("residency", false, in.RecheckErr.Error())
		} else {
			add("residency", true, "")
		}
		if cur, err := BuildPlan(in.Rec); err != nil {
			add("plan_current", false, err.Error())
		} else if cur.Kind != j.Kind || !reflect.DeepEqual(cur.Plan, j.Plan) || cur.RollbackPlan != j.RollbackPlan {
			add("plan_current", false, "the plan no longer matches what the planner builds from the recommendation today: plan again")
		} else {
			add("plan_current", true, "")
		}
	}

	bad := ""
	var run, held []string
	for _, c := range j.Plan.CLI {
		if m := heldRe.FindStringSubmatch(c); m != nil {
			if !commandRe.MatchString(m[1]) {
				bad = c
				break
			}
			held = append(held, c)
			continue
		}
		if !commandRe.MatchString(c) {
			bad = c
			break
		}
		run = append(run, c)
	}
	if bad != "" {
		add("commands_safe", false, "a command has an unexpected shape and is not shown as runnable")
	} else {
		add("commands_safe", true, "")
	}

	rep.Ready = j.Status == Approved
	for _, c := range rep.Checks {
		rep.Ready = rep.Ready && c.OK
	}
	rep.Steps = append(rep.Steps, j.Plan.Steps...)
	if bad == "" {
		rep.WouldRun = append(rep.WouldRun, run...)
		rep.Held = append(rep.Held, held...)
	}
	return rep
}

// ErrDryRunUnavailable is returned by a real executor that has not been wired; only the dry run exists (ADR-0048).
var ErrDryRunUnavailable = errors.New("automation: this platform only supports dry runs")
