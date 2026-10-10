package domain

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"
)

// Team owns projects. A project belongs to at most one team.
type Team struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
}

// EnvironmentKinds are the kinds an environment can be (production is the one that carries risk).
var EnvironmentKinds = map[string]bool{"production": true, "staging": true, "development": true, "test": true, "other": true}

// Environment is a place a project runs in: the cloud connections assigned to it carry its cost and carbon.
type Environment struct {
	ID            string    `json:"id"`
	ProjectID     string    `json:"project_id"`
	Name          string    `json:"name"`
	Kind          string    `json:"kind"`
	ConnectionIDs []string  `json:"connection_ids"`
	CreatedAt     time.Time `json:"created_at"`
}

// AllocationTarget is one receiver of a shared cost and its weight (weights are relative, not percentages).
type AllocationTarget struct {
	ProjectID string  `json:"project_id"`
	Weight    float64 `json:"weight"`
}

// AllocationRule splits what SourceProjectID is billed for ServiceName (empty = every service without a rule of its
// own) between its targets. A target may be the source itself: it keeps its share.
type AllocationRule struct {
	ID              string             `json:"id"`
	Name            string             `json:"name"`
	SourceProjectID string             `json:"source_project_id"`
	ServiceName     string             `json:"service_name"`
	Targets         []AllocationTarget `json:"targets"`
	CreatedAt       time.Time          `json:"created_at"`
}

const (
	MaxRuleTargets = 20
	maxWeight      = 1_000_000
)

// Validate checks a rule before it is stored.
func (r AllocationRule) Validate() error {
	if n := len([]rune(strings.TrimSpace(r.Name))); n < 1 || n > 120 {
		return fmt.Errorf("%w: rule name must be 1-120 characters", ErrInvalidAllocation)
	}
	if len([]rune(r.ServiceName)) > 200 {
		return fmt.Errorf("%w: service name is too long", ErrInvalidAllocation)
	}
	if r.SourceProjectID == "" {
		return fmt.Errorf("%w: a source project is required", ErrInvalidAllocation)
	}
	if len(r.Targets) == 0 || len(r.Targets) > MaxRuleTargets {
		return fmt.Errorf("%w: a rule needs 1 to %d targets", ErrInvalidAllocation, MaxRuleTargets)
	}
	seen := map[string]bool{}
	for _, t := range r.Targets {
		switch {
		case t.ProjectID == "":
			return fmt.Errorf("%w: every target needs a project", ErrInvalidAllocation)
		case seen[t.ProjectID]:
			return fmt.Errorf("%w: a project can be a target once", ErrInvalidAllocation)
		case math.IsNaN(t.Weight) || t.Weight <= 0 || t.Weight > maxWeight:
			return fmt.Errorf("%w: weights must be greater than zero", ErrInvalidAllocation)
		}
		seen[t.ProjectID] = true
	}
	return nil
}

// CostLine is the cost one project was billed for one service in the window, in the report currency.
type CostLine struct {
	ProjectID, Service string
	Cost               float64
}

// ProjectCost is a project's cost before and after the rules.
type ProjectCost struct {
	ProjectID    string  `json:"project_id"`
	Direct       float64 `json:"direct"`        // billed to the project's own connections
	AllocatedIn  float64 `json:"allocated_in"`  // received from rules of other projects
	AllocatedOut float64 `json:"allocated_out"` // handed to targets of its own rules
	Total        float64 `json:"total"`         // Direct - AllocatedOut + AllocatedIn
}

// Allocate applies the rules to the cost lines. The sum of the totals always equals the sum of the direct costs: a
// rule moves cost, it never creates or drops any. A line is matched by the most specific rule of its project (the
// rule for its service, else the rule for everything).
func Allocate(lines []CostLine, rules []AllocationRule) []ProjectCost {
	type key struct{ project, service string }
	byKey := map[key]AllocationRule{}
	for _, r := range rules {
		byKey[key{r.SourceProjectID, r.ServiceName}] = r
	}
	acc := map[string]*ProjectCost{}
	get := func(id string) *ProjectCost {
		if acc[id] == nil {
			acc[id] = &ProjectCost{ProjectID: id}
		}
		return acc[id]
	}
	for _, l := range lines {
		get(l.ProjectID).Direct += l.Cost
		rule, ok := byKey[key{l.ProjectID, l.Service}]
		if !ok {
			rule, ok = byKey[key{l.ProjectID, ""}]
		}
		if !ok {
			continue
		}
		var sum float64
		for _, t := range rule.Targets {
			sum += t.Weight
		}
		if sum <= 0 {
			continue
		}
		for _, t := range rule.Targets {
			share := l.Cost * t.Weight / sum
			if t.ProjectID == l.ProjectID {
				continue // the source keeps its own share: nothing moves
			}
			get(l.ProjectID).AllocatedOut += share
			get(t.ProjectID).AllocatedIn += share
		}
	}
	out := make([]ProjectCost, 0, len(acc))
	for _, p := range acc {
		p.Total = p.Direct - p.AllocatedOut + p.AllocatedIn
		out = append(out, *p)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Total != out[j].Total {
			return out[i].Total > out[j].Total
		}
		return out[i].ProjectID < out[j].ProjectID
	})
	return out
}

// EnvironmentCost is the cost billed to one environment of a project (EnvironmentID nil: connections of the project
// that are not assigned to any environment). Allocation rules move cost between projects, not between environments.
type EnvironmentCost struct {
	ProjectID     string  `json:"project_id"`
	EnvironmentID *string `json:"environment_id"`
	Cost          float64 `json:"cost"`
}

// TeamCost is the total, after allocation, of the projects a team owns (TeamID nil: projects without a team).
type TeamCost struct {
	TeamID *string `json:"team_id"`
	Total  float64 `json:"total"`
}

// AllocationReport is the cost of a period by project, environment and team, with the rules applied.
type AllocationReport struct {
	Currency     string            `json:"currency"`
	From         string            `json:"from"`
	To           string            `json:"to"`
	Projects     []ProjectCost     `json:"projects"`
	Environments []EnvironmentCost `json:"environments"`
	Teams        []TeamCost        `json:"teams"`
	// Excluded counts amounts left out because their currency has no reference rate for the day.
	Excluded int `json:"excluded"`
}

// StructureRepository stores teams, environments and allocation rules, and reads the cost lines they are applied to.
type StructureRepository interface {
	ListTeams(ctx context.Context, tenantID string) ([]Team, error)
	CreateTeam(ctx context.Context, tenantID, name string) (Team, error)
	DeleteTeam(ctx context.Context, tenantID, id string) error
	SetProjectTeam(ctx context.Context, tenantID, projectID string, teamID *string) error

	ListEnvironments(ctx context.Context, tenantID string) ([]Environment, error)
	CreateEnvironment(ctx context.Context, tenantID, projectID, name, kind string) (Environment, error)
	DeleteEnvironment(ctx context.Context, tenantID, id string) error
	// SetEnvironmentConnections replaces the environment's connections. It fails with ErrInvalidAllocation when one of
	// them does not belong to the environment's project.
	SetEnvironmentConnections(ctx context.Context, tenantID, environmentID string, connectionIDs []string) error

	ListAllocationRules(ctx context.Context, tenantID string) ([]AllocationRule, error)
	CreateAllocationRule(ctx context.Context, tenantID string, r AllocationRule) (AllocationRule, error)
	DeleteAllocationRule(ctx context.Context, tenantID, id string) error

	// CostLines returns the cost of [from, to) per project, environment and service in the report currency (the
	// organization's, else the window's dominant one) and the count of amounts left out for lack of a rate.
	CostLines(ctx context.Context, tenantID string, from, to time.Time) (currency string, lines []EnvironmentLine, excluded int, err error)
}

// EnvironmentLine is a cost line that also knows the environment of the connection that billed it.
type EnvironmentLine struct {
	CostLine
	EnvironmentID *string
}
