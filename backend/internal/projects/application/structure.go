package application

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/quixgit/greenops-reliabilix/backend/internal/projects/domain"
)

// ErrStructureUnavailable: the service was built without a structure repository (tests of other features).
var ErrStructureUnavailable = errors.New("organization structure is not available")

func (s Service) structure() (domain.StructureRepository, error) {
	if s.Structure == nil {
		return nil, ErrStructureUnavailable
	}
	return s.Structure, nil
}

func (s Service) ListTeams(ctx context.Context, tenantID string) ([]domain.Team, error) {
	r, err := s.structure()
	if err != nil {
		return nil, err
	}
	return r.ListTeams(ctx, tenantID)
}

func (s Service) CreateTeam(ctx context.Context, tenantID, name string) (domain.Team, error) {
	r, err := s.structure()
	if err != nil {
		return domain.Team{}, err
	}
	name = strings.TrimSpace(name)
	if n := len([]rune(name)); n < 1 || n > 80 {
		return domain.Team{}, fmt.Errorf("%w: team name must be 1-80 characters", domain.ErrInvalidAllocation)
	}
	return r.CreateTeam(ctx, tenantID, name)
}

func (s Service) DeleteTeam(ctx context.Context, tenantID, id string) error {
	r, err := s.structure()
	if err != nil {
		return err
	}
	return r.DeleteTeam(ctx, tenantID, id)
}

// SetProjectTeam assigns the project to a team (nil clears it).
func (s Service) SetProjectTeam(ctx context.Context, tenantID, projectID string, teamID *string) error {
	r, err := s.structure()
	if err != nil {
		return err
	}
	return r.SetProjectTeam(ctx, tenantID, projectID, teamID)
}

func (s Service) ListEnvironments(ctx context.Context, tenantID string) ([]domain.Environment, error) {
	r, err := s.structure()
	if err != nil {
		return nil, err
	}
	return r.ListEnvironments(ctx, tenantID)
}

func (s Service) CreateEnvironment(ctx context.Context, tenantID, projectID, name, kind string) (domain.Environment, error) {
	r, err := s.structure()
	if err != nil {
		return domain.Environment{}, err
	}
	name = strings.TrimSpace(name)
	if n := len([]rune(name)); n < 1 || n > 60 {
		return domain.Environment{}, fmt.Errorf("%w: environment name must be 1-60 characters", domain.ErrInvalidAllocation)
	}
	if !domain.EnvironmentKinds[kind] {
		return domain.Environment{}, fmt.Errorf("%w: unknown environment kind", domain.ErrInvalidAllocation)
	}
	return r.CreateEnvironment(ctx, tenantID, projectID, name, kind)
}

func (s Service) DeleteEnvironment(ctx context.Context, tenantID, id string) error {
	r, err := s.structure()
	if err != nil {
		return err
	}
	return r.DeleteEnvironment(ctx, tenantID, id)
}

// SetEnvironmentConnections replaces the connections of an environment (de-duplicated).
func (s Service) SetEnvironmentConnections(ctx context.Context, tenantID, environmentID string, ids []string) error {
	r, err := s.structure()
	if err != nil {
		return err
	}
	seen := map[string]bool{}
	uniq := make([]string, 0, len(ids))
	for _, id := range ids {
		if !seen[id] {
			seen[id] = true
			uniq = append(uniq, id)
		}
	}
	if len(uniq) > 100 {
		return fmt.Errorf("%w: at most 100 connections per environment", domain.ErrInvalidAllocation)
	}
	return r.SetEnvironmentConnections(ctx, tenantID, environmentID, uniq)
}

func (s Service) ListAllocationRules(ctx context.Context, tenantID string) ([]domain.AllocationRule, error) {
	r, err := s.structure()
	if err != nil {
		return nil, err
	}
	return r.ListAllocationRules(ctx, tenantID)
}

func (s Service) CreateAllocationRule(ctx context.Context, tenantID string, rule domain.AllocationRule) (domain.AllocationRule, error) {
	r, err := s.structure()
	if err != nil {
		return domain.AllocationRule{}, err
	}
	rule.Name, rule.ServiceName = strings.TrimSpace(rule.Name), strings.TrimSpace(rule.ServiceName)
	if err := rule.Validate(); err != nil {
		return domain.AllocationRule{}, err
	}
	return r.CreateAllocationRule(ctx, tenantID, rule)
}

func (s Service) DeleteAllocationRule(ctx context.Context, tenantID, id string) error {
	r, err := s.structure()
	if err != nil {
		return err
	}
	return r.DeleteAllocationRule(ctx, tenantID, id)
}

// Allocation reports the cost of [from, to) by project, environment and team with the allocation rules applied.
func (s Service) Allocation(ctx context.Context, tenantID string, from, to time.Time) (domain.AllocationReport, error) {
	r, err := s.structure()
	if err != nil {
		return domain.AllocationReport{}, err
	}
	if !from.Before(to) || to.Sub(from) > 400*24*time.Hour {
		return domain.AllocationReport{}, domain.ErrInvalidPeriod
	}
	currency, env, excluded, err := r.CostLines(ctx, tenantID, from, to)
	if err != nil {
		return domain.AllocationReport{}, err
	}
	rules, err := r.ListAllocationRules(ctx, tenantID)
	if err != nil {
		return domain.AllocationReport{}, err
	}
	projects, err := s.Repo.List(ctx, tenantID)
	if err != nil {
		return domain.AllocationReport{}, err
	}

	lines := make([]domain.CostLine, 0, len(env))
	type envKey struct {
		project string
		env     string // "" = unassigned
	}
	byEnv := map[envKey]float64{}
	for _, l := range env {
		lines = append(lines, l.CostLine)
		k := envKey{project: l.ProjectID}
		if l.EnvironmentID != nil {
			k.env = *l.EnvironmentID
		}
		byEnv[k] += l.Cost
	}
	rep := domain.AllocationReport{Currency: currency, From: from.Format(time.DateOnly), To: to.Format(time.DateOnly),
		Projects: domain.Allocate(lines, rules), Environments: []domain.EnvironmentCost{}, Teams: []domain.TeamCost{}, Excluded: excluded}
	for k, c := range byEnv {
		e := domain.EnvironmentCost{ProjectID: k.project, Cost: c}
		if k.env != "" {
			id := k.env
			e.EnvironmentID = &id
		}
		rep.Environments = append(rep.Environments, e)
	}
	team := map[string]*string{}
	for _, p := range projects {
		team[p.ID] = p.TeamID
	}
	totals := map[string]float64{}
	var noTeam float64
	for _, p := range rep.Projects {
		if t := team[p.ProjectID]; t != nil {
			totals[*t] += p.Total
		} else {
			noTeam += p.Total
		}
	}
	for id, total := range totals {
		id := id
		rep.Teams = append(rep.Teams, domain.TeamCost{TeamID: &id, Total: total})
	}
	if noTeam != 0 {
		rep.Teams = append(rep.Teams, domain.TeamCost{Total: noTeam})
	}
	return rep, nil
}
