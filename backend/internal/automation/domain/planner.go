package domain

import (
	"fmt"
	"regexp"
	"strings"
)

// Plans contain shell commands that a person will copy and run. Every value interpolated into them comes from
// the database but originates in cloud provider data, so each one is validated against a strict pattern first:
// a value that does not match is an error, never an escaped string.
var (
	instanceIDRe   = regexp.MustCompile(`^i-[0-9a-f]{8,17}$`)
	instanceTypeRe = regexp.MustCompile(`^[a-z][a-z0-9]*(?:-[a-z0-9]+)?\.[a-z0-9]+$`)
	regionRe       = regexp.MustCompile(`^[a-z]{2}(?:-[a-z]+)+-\d$`) // real AWS region: used inside commands
	// planRegionRe is the looser shape allowed in plans that contain no commands (region shift is a checklist):
	// plain lower-case identifiers only, no whitespace or shell metacharacters.
	planRegionRe = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{1,30}$`)
	familyRe     = regexp.MustCompile(`^([a-z]+)(\d+)([a-z-]*)$`)
)

// PlanResult is the full output of planning.
type PlanResult struct {
	Kind         Kind
	Plan         Plan
	Risk         Risk
	RollbackPlan string
}

// BuildPlan derives a reviewable change plan from an approved recommendation. It performs no I/O and no change.
func BuildPlan(r RecommendationInfo) (PlanResult, error) {
	switch r.Type {
	case "rightsizing":
		switch r.Details["action"] {
		case "modify":
			return resizePlan(r)
		case "terminate":
			return terminatePlan(r)
		}
		return PlanResult{}, fmt.Errorf("%w: unknown rightsizing action", ErrInvalidInput)
	case "region_shift":
		return regionShiftPlan(r)
	}
	return PlanResult{}, fmt.Errorf("%w: %s", ErrUnsupported, r.Type)
}

// instanceFacts validates the fields that end up inside commands.
func instanceFacts(r RecommendationInfo, needTarget bool) (id, region, cur, target string, err error) {
	id, region, cur, target = r.Details["resource_id"], r.CurrentRegion, r.Details["current_type"], r.Details["target_type"]
	switch {
	case !instanceIDRe.MatchString(id):
		err = fmt.Errorf("%w: resource id is not an EC2 instance id", ErrInvalidInput)
	case !regionRe.MatchString(region):
		err = fmt.Errorf("%w: region is not a valid AWS region", ErrInvalidInput)
	case !instanceTypeRe.MatchString(cur):
		err = fmt.Errorf("%w: current instance type is invalid", ErrInvalidInput)
	case needTarget && !instanceTypeRe.MatchString(target):
		err = fmt.Errorf("%w: target instance type is invalid", ErrInvalidInput)
	}
	return
}

// attributes splits "m6gd" into the generation attributes ("gd"); ok=false for names it does not understand.
func attributes(instanceType string) (string, bool) {
	family, _, _ := strings.Cut(instanceType, ".")
	m := familyRe.FindStringSubmatch(family)
	if m == nil {
		return "", false
	}
	return m[3], true
}

func isArm(attrs string) bool        { return strings.Contains(attrs, "g") }
func hasLocalDisk(attrs string) bool { return strings.Contains(attrs, "d") }

func resizeCommands(region, id, to string) []string {
	return []string{
		fmt.Sprintf("aws ec2 stop-instances --region %s --instance-ids %s", region, id),
		fmt.Sprintf("aws ec2 wait instance-stopped --region %s --instance-ids %s", region, id),
		fmt.Sprintf(`aws ec2 modify-instance-attribute --region %s --instance-id %s --instance-type '{"Value":"%s"}'`, region, id, to),
		fmt.Sprintf("aws ec2 start-instances --region %s --instance-ids %s", region, id),
		fmt.Sprintf("aws ec2 wait instance-status-ok --region %s --instance-ids %s", region, id),
	}
}

func resizePlan(r RecommendationInfo) (PlanResult, error) {
	id, region, cur, target, err := instanceFacts(r, true)
	if err != nil {
		return PlanResult{}, err
	}
	risk := Risk{Level: RiskMedium, Factors: []string{
		"The instance must be stopped and started: expect downtime.",
		"A public IP address that is not an Elastic IP changes after the restart.",
	}}
	curAttr, curOK := attributes(cur)
	tgtAttr, tgtOK := attributes(target)
	if curOK && tgtOK {
		if isArm(curAttr) != isArm(tgtAttr) {
			risk.Level = RiskHigh
			risk.Factors = append(risk.Factors, "The change crosses CPU architectures (x86 <-> Arm): the instance's AMI and software must support the target.")
		}
		if hasLocalDisk(curAttr) {
			risk.Level = RiskHigh
			risk.Factors = append(risk.Factors, "The current type has local instance-store disks: their data is lost when the instance stops.")
		}
	} else {
		risk.Level = RiskHigh
		risk.Factors = append(risk.Factors, "The instance family could not be analysed: review compatibility manually.")
	}
	return PlanResult{
		Kind: EC2Resize,
		Plan: Plan{
			Summary: fmt.Sprintf("Change instance %s from %s to %s in %s.", id, cur, target, region),
			Steps: []string{
				"Confirm a maintenance window and that the workload tolerates a restart.",
				"Take a backup (AMI or EBS snapshots) of the instance.",
				"Stop the instance, change its type, start it.",
				"Check application health and the instance status checks.",
				"Watch utilization for a few days; keep the rollback plan until the change is proven.",
			},
			Terraform: fmt.Sprintf("# in the aws_instance resource of %s (apply with a plan review)\ninstance_type = %q # was %q", id, target, cur),
			CLI:       resizeCommands(region, id, target),
		},
		Risk:         risk,
		RollbackPlan: fmt.Sprintf("Run the same stop / modify-instance-attribute / start sequence with --instance-type %s to restore the original size.", cur),
	}, nil
}

func terminatePlan(r RecommendationInfo) (PlanResult, error) {
	id, region, cur, _, err := instanceFacts(r, false)
	if err != nil {
		return PlanResult{}, err
	}
	return PlanResult{
		Kind: EC2Terminate,
		Plan: Plan{
			Summary: fmt.Sprintf("Retire instance %s (%s) in %s.", id, cur, region),
			Steps: []string{
				"Confirm with the owner that the instance is really unused (check connections, cron jobs, load balancer and auto scaling membership).",
				"Create an image of the instance so it can be rebuilt.",
				"Stop the instance and observe for an agreed period before terminating.",
				"Terminate only after that period; check that attached volumes are not needed.",
			},
			CLI: []string{
				fmt.Sprintf(`aws ec2 create-image --region %s --instance-id %s --name "reliabilix-pre-retire-%s" --no-reboot`, region, id, id),
				fmt.Sprintf("aws ec2 stop-instances --region %s --instance-ids %s", region, id),
				fmt.Sprintf("# after the observation period: aws ec2 terminate-instances --region %s --instance-ids %s", region, id),
			},
			Terraform: fmt.Sprintf("# remove the aws_instance resource of %s from the configuration and apply after review", id),
		},
		Risk: Risk{Level: RiskHigh, Factors: []string{
			"Termination is irreversible: data on instance-store disks and on volumes set to delete on termination is lost.",
			"The instance may be reachable through addresses, DNS records or security groups that other systems rely on.",
		}},
		RollbackPlan: "While the instance is only stopped: start it again. After termination: launch a new instance from the image created in step 2 and restore any needed volumes from snapshots.",
	}, nil
}

func regionShiftPlan(r RecommendationInfo) (PlanResult, error) {
	if !planRegionRe.MatchString(r.CurrentRegion) || !planRegionRe.MatchString(r.RecommendedRegion) || r.CurrentRegion == r.RecommendedRegion {
		return PlanResult{}, fmt.Errorf("%w: region shift needs two different valid regions", ErrInvalidInput)
	}
	return PlanResult{
		Kind: RegionShift,
		Plan: Plan{
			Summary: fmt.Sprintf("Move the workload from %s to %s.", r.CurrentRegion, r.RecommendedRegion),
			Steps: []string{
				"Confirm the target region is allowed by the project's data-residency policy and by contracts.",
				"Estimate data-transfer cost and the latency impact on users and dependent services.",
				"Provision the stack in the target region (infrastructure as code), with a copy of required data.",
				"Run both regions in parallel, shift traffic gradually and verify.",
				"Decommission the source region only after a successful observation period.",
			},
			Terraform: fmt.Sprintf("# add a provider alias for %s and move the module's region from %s", r.RecommendedRegion, r.CurrentRegion),
		},
		Risk: Risk{Level: RiskHigh, Factors: []string{
			"Moving data and traffic between regions can change latency, availability and data-transfer cost.",
			"Data-residency obligations must be re-checked at execution time.",
			"Stateful services need a migration and consistency plan.",
		}},
		RollbackPlan: fmt.Sprintf("Keep the %s environment running until the new region is verified; roll back by shifting traffic back to it.", r.CurrentRegion),
	}, nil
}
