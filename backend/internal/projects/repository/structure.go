package repository

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/audit"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/database"
	"github.com/quixgit/greenops-reliabilix/backend/internal/projects/domain"
	"github.com/quixgit/greenops-reliabilix/backend/internal/projects/repository/db"
)

// isForeignKeyViolation: a referenced project, team or connection does not exist (or belongs to another organization,
// which row-level security makes look the same).
func isForeignKeyViolation(err error) bool {
	var pg *pgconn.PgError
	return errors.As(err, &pg) && pg.Code == "23503"
}

func (r Postgres) ListTeams(ctx context.Context, tenantID string) ([]domain.Team, error) {
	out := []domain.Team{}
	err := database.WithTenantTx(ctx, r.Pool, tenantID, func(tx pgx.Tx) error {
		rows, err := db.New(tx).ListTeams(ctx, tenantID)
		for _, t := range rows {
			out = append(out, domain.Team{ID: t.ID, Name: t.Name, CreatedAt: t.CreatedAt})
		}
		return err
	})
	return out, err
}

func (r Postgres) CreateTeam(ctx context.Context, tenantID, name string) (domain.Team, error) {
	var t domain.Team
	err := database.WithTenantTx(ctx, r.Pool, tenantID, func(tx pgx.Tx) error {
		row, err := db.New(tx).CreateTeam(ctx, db.CreateTeamParams{TenantID: tenantID, Name: name})
		if err != nil {
			return err
		}
		t = domain.Team{ID: row.ID, Name: row.Name, CreatedAt: row.CreatedAt}
		return audit.Record(ctx, tx, "team.created", "team:"+t.ID, map[string]any{"name": name})
	})
	if database.IsUniqueViolation(err) {
		return domain.Team{}, domain.ErrDuplicate
	}
	return t, err
}

func (r Postgres) DeleteTeam(ctx context.Context, tenantID, id string) error {
	return database.WithTenantTx(ctx, r.Pool, tenantID, func(tx pgx.Tx) error {
		n, err := db.New(tx).DeleteTeam(ctx, db.DeleteTeamParams{TenantID: tenantID, ID: id})
		if err != nil {
			return err
		}
		if n == 0 {
			return domain.ErrNotFound
		}
		return audit.Record(ctx, tx, "team.deleted", "team:"+id, nil)
	})
}

func (r Postgres) SetProjectTeam(ctx context.Context, tenantID, projectID string, teamID *string) error {
	err := database.WithTenantTx(ctx, r.Pool, tenantID, func(tx pgx.Tx) error {
		n, err := db.New(tx).SetProjectTeam(ctx, db.SetProjectTeamParams{TenantID: tenantID, ID: projectID, TeamID: teamID})
		if err != nil {
			return err
		}
		if n == 0 {
			return domain.ErrNotFound
		}
		return audit.Record(ctx, tx, "project.team_set", "project:"+projectID, map[string]any{"team_id": teamID})
	})
	if isForeignKeyViolation(err) {
		return domain.ErrNotFound
	}
	return err
}

func (r Postgres) ListEnvironments(ctx context.Context, tenantID string) ([]domain.Environment, error) {
	out := []domain.Environment{}
	err := database.WithTenantTx(ctx, r.Pool, tenantID, func(tx pgx.Tx) error {
		rows, err := db.New(tx).ListEnvironments(ctx, tenantID)
		for _, e := range rows {
			ids := e.ConnectionIds
			if ids == nil {
				ids = []string{}
			}
			out = append(out, domain.Environment{ID: e.ID, ProjectID: e.ProjectID, Name: e.Name, Kind: e.Kind, ConnectionIDs: ids, CreatedAt: e.CreatedAt})
		}
		return err
	})
	return out, err
}

func (r Postgres) CreateEnvironment(ctx context.Context, tenantID, projectID, name, kind string) (domain.Environment, error) {
	var e domain.Environment
	err := database.WithTenantTx(ctx, r.Pool, tenantID, func(tx pgx.Tx) error {
		row, err := db.New(tx).CreateEnvironment(ctx, db.CreateEnvironmentParams{TenantID: tenantID, ProjectID: projectID, Name: name, Kind: kind})
		if err != nil {
			return err
		}
		e = domain.Environment{ID: row.ID, ProjectID: row.ProjectID, Name: row.Name, Kind: row.Kind, ConnectionIDs: []string{}, CreatedAt: row.CreatedAt}
		return audit.Record(ctx, tx, "environment.created", "environment:"+e.ID, map[string]any{"project_id": projectID, "name": name, "kind": kind})
	})
	switch {
	case database.IsUniqueViolation(err):
		return domain.Environment{}, domain.ErrDuplicate
	case isForeignKeyViolation(err):
		return domain.Environment{}, domain.ErrNotFound
	}
	return e, err
}

func (r Postgres) DeleteEnvironment(ctx context.Context, tenantID, id string) error {
	return database.WithTenantTx(ctx, r.Pool, tenantID, func(tx pgx.Tx) error {
		n, err := db.New(tx).DeleteEnvironment(ctx, db.DeleteEnvironmentParams{TenantID: tenantID, ID: id})
		if err != nil {
			return err
		}
		if n == 0 {
			return domain.ErrNotFound
		}
		return audit.Record(ctx, tx, "environment.deleted", "environment:"+id, nil)
	})
}

func (r Postgres) SetEnvironmentConnections(ctx context.Context, tenantID, environmentID string, connectionIDs []string) error {
	return database.WithTenantTx(ctx, r.Pool, tenantID, func(tx pgx.Tx) error {
		q := db.New(tx)
		env, err := q.GetEnvironment(ctx, db.GetEnvironmentParams{TenantID: tenantID, ID: environmentID})
		if database.IsNotFound(err) {
			return domain.ErrNotFound
		}
		if err != nil {
			return err
		}
		if err := q.ClearEnvironmentConnections(ctx, db.ClearEnvironmentConnectionsParams{TenantID: tenantID, EnvironmentID: environmentID}); err != nil {
			return err
		}
		if len(connectionIDs) > 0 {
			n, err := q.AddEnvironmentConnections(ctx, db.AddEnvironmentConnectionsParams{
				TenantID: tenantID, EnvironmentID: environmentID, ProjectID: env.ProjectID, ConnectionIds: connectionIDs})
			if err != nil {
				return err
			}
			if int(n) != len(connectionIDs) {
				return domain.ErrInvalidAllocation // rolls the whole change back
			}
		}
		return audit.Record(ctx, tx, "environment.connections_set", "environment:"+environmentID, map[string]any{"connections": len(connectionIDs)})
	})
}

func (r Postgres) ListAllocationRules(ctx context.Context, tenantID string) ([]domain.AllocationRule, error) {
	out := []domain.AllocationRule{}
	err := database.WithTenantTx(ctx, r.Pool, tenantID, func(tx pgx.Tx) error {
		q := db.New(tx)
		rules, err := q.ListAllocationRules(ctx, tenantID)
		if err != nil {
			return err
		}
		targets, err := q.ListAllocationTargets(ctx, tenantID)
		if err != nil {
			return err
		}
		byRule := map[string][]domain.AllocationTarget{}
		for _, t := range targets {
			byRule[t.RuleID] = append(byRule[t.RuleID], domain.AllocationTarget{ProjectID: t.ProjectID, Weight: t.Weight})
		}
		for _, x := range rules {
			out = append(out, domain.AllocationRule{ID: x.ID, Name: x.Name, SourceProjectID: x.SourceProjectID, ServiceName: x.ServiceName,
				Targets: byRule[x.ID], CreatedAt: x.CreatedAt})
		}
		return nil
	})
	return out, err
}

func (r Postgres) CreateAllocationRule(ctx context.Context, tenantID string, rule domain.AllocationRule) (domain.AllocationRule, error) {
	err := database.WithTenantTx(ctx, r.Pool, tenantID, func(tx pgx.Tx) error {
		q := db.New(tx)
		row, err := q.CreateAllocationRule(ctx, db.CreateAllocationRuleParams{TenantID: tenantID, Name: rule.Name,
			SourceProjectID: rule.SourceProjectID, ServiceName: rule.ServiceName})
		if err != nil {
			return err
		}
		for _, t := range rule.Targets {
			if err := q.CreateAllocationTarget(ctx, db.CreateAllocationTargetParams{RuleID: row.ID, TenantID: tenantID, ProjectID: t.ProjectID, Weight: t.Weight}); err != nil {
				return err
			}
		}
		rule.ID, rule.CreatedAt = row.ID, row.CreatedAt
		return audit.Record(ctx, tx, "allocation_rule.created", "allocation_rule:"+row.ID,
			map[string]any{"name": rule.Name, "source_project_id": rule.SourceProjectID, "service": rule.ServiceName, "targets": len(rule.Targets)})
	})
	switch {
	case database.IsUniqueViolation(err):
		return domain.AllocationRule{}, domain.ErrDuplicate
	case isForeignKeyViolation(err):
		return domain.AllocationRule{}, domain.ErrNotFound
	}
	return rule, err
}

func (r Postgres) DeleteAllocationRule(ctx context.Context, tenantID, id string) error {
	return database.WithTenantTx(ctx, r.Pool, tenantID, func(tx pgx.Tx) error {
		n, err := db.New(tx).DeleteAllocationRule(ctx, db.DeleteAllocationRuleParams{TenantID: tenantID, ID: id})
		if err != nil {
			return err
		}
		if n == 0 {
			return domain.ErrNotFound
		}
		return audit.Record(ctx, tx, "allocation_rule.deleted", "allocation_rule:"+id, nil)
	})
}

func (r Postgres) CostLines(ctx context.Context, tenantID string, from, to time.Time) (string, []domain.EnvironmentLine, int, error) {
	var (
		currency string
		lines    []domain.EnvironmentLine
		excluded int
	)
	err := database.WithTenantTx(ctx, r.Pool, tenantID, func(tx pgx.Tx) error {
		q := db.New(tx)
		var err error
		if currency, err = q.AllocationCurrency(ctx, db.AllocationCurrencyParams{TenantID: tenantID, FromTs: from, ToTs: to}); err != nil {
			return err
		}
		if currency == "" {
			return nil // no usage in the window
		}
		rows, err := q.ProjectServiceCost(ctx, db.ProjectServiceCostParams{TenantID: tenantID, Currency: currency, FromTs: from, ToTs: to})
		for _, x := range rows {
			lines = append(lines, domain.EnvironmentLine{CostLine: domain.CostLine{ProjectID: x.ProjectID, Service: x.ServiceName, Cost: x.Cost}, EnvironmentID: x.EnvironmentID})
			excluded += int(x.Excluded)
		}
		return err
	})
	return currency, lines, excluded, err
}
