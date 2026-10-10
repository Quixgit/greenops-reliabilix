-- +goose Up
-- Organization structure: teams that own projects, environments of a project (the cloud connections that run it) and
-- allocation rules that split the cost of shared services between projects. Cost is never rewritten: the rules are
-- applied when the allocation report is read, so changing a rule changes the report for any period at once.

-- Every reference below is a composite (tenant_id, id) foreign key: a plain foreign key is checked without row-level
-- security, so it would accept the id of another organization's project, team or connection.
ALTER TABLE projects.projects ADD CONSTRAINT projects_tenant_id_id_key UNIQUE (tenant_id, id);
ALTER TABLE cloudaccounts.connections ADD CONSTRAINT connections_tenant_id_id_key UNIQUE (tenant_id, id);

CREATE TABLE projects.teams (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id  uuid NOT NULL REFERENCES tenants.organizations(id),
    name       text NOT NULL CHECK (char_length(name) BETWEEN 1 AND 80),
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, name),
    UNIQUE (tenant_id, id)
);
ALTER TABLE projects.projects ADD COLUMN team_id uuid;
ALTER TABLE projects.projects ADD CONSTRAINT projects_team_fk FOREIGN KEY (tenant_id, team_id)
    REFERENCES projects.teams (tenant_id, id) ON DELETE SET NULL (team_id);

CREATE TABLE projects.environments (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id  uuid NOT NULL REFERENCES tenants.organizations(id),
    project_id uuid NOT NULL,
    name       text NOT NULL CHECK (char_length(name) BETWEEN 1 AND 60),
    kind       text NOT NULL CHECK (kind IN ('production','staging','development','test','other')),
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (project_id, name),
    UNIQUE (tenant_id, id),
    FOREIGN KEY (tenant_id, project_id) REFERENCES projects.projects (tenant_id, id) ON DELETE CASCADE
);
-- A connection runs in at most one environment: its cost (and carbon) is that environment's.
CREATE TABLE projects.environment_connections (
    tenant_id      uuid NOT NULL REFERENCES tenants.organizations(id),
    environment_id uuid NOT NULL,
    connection_id  uuid NOT NULL UNIQUE,
    PRIMARY KEY (environment_id, connection_id),
    FOREIGN KEY (tenant_id, environment_id) REFERENCES projects.environments (tenant_id, id) ON DELETE CASCADE,
    FOREIGN KEY (tenant_id, connection_id) REFERENCES cloudaccounts.connections (tenant_id, id) ON DELETE CASCADE
);

-- Shared cost: what the source project is billed for a service (or for everything, service_name = '') is split between
-- the target projects by weight. A rule for one service wins over the project's rule for everything.
CREATE TABLE projects.allocation_rules (
    id                uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id         uuid NOT NULL REFERENCES tenants.organizations(id),
    name              text NOT NULL CHECK (char_length(name) BETWEEN 1 AND 120),
    source_project_id uuid NOT NULL,
    service_name      text NOT NULL DEFAULT '' CHECK (char_length(service_name) <= 200),
    created_at        timestamptz NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, id),
    FOREIGN KEY (tenant_id, source_project_id) REFERENCES projects.projects (tenant_id, id) ON DELETE CASCADE
);
CREATE UNIQUE INDEX allocation_rules_source_service ON projects.allocation_rules (tenant_id, source_project_id, service_name);
CREATE TABLE projects.allocation_targets (
    rule_id    uuid NOT NULL,
    tenant_id  uuid NOT NULL REFERENCES tenants.organizations(id),
    project_id uuid NOT NULL,
    weight     numeric(12,4) NOT NULL CHECK (weight > 0),
    PRIMARY KEY (rule_id, project_id),
    FOREIGN KEY (tenant_id, rule_id) REFERENCES projects.allocation_rules (tenant_id, id) ON DELETE CASCADE,
    FOREIGN KEY (tenant_id, project_id) REFERENCES projects.projects (tenant_id, id) ON DELETE CASCADE
);

SELECT platform.enable_tenant_rls('projects.teams', 'tenant_id');
SELECT platform.enable_tenant_rls('projects.environments', 'tenant_id');
SELECT platform.enable_tenant_rls('projects.environment_connections', 'tenant_id');
SELECT platform.enable_tenant_rls('projects.allocation_rules', 'tenant_id');
SELECT platform.enable_tenant_rls('projects.allocation_targets', 'tenant_id');

GRANT SELECT, INSERT, UPDATE, DELETE ON projects.teams, projects.environments, projects.environment_connections,
  projects.allocation_rules, projects.allocation_targets TO greenops_api;
GRANT SELECT ON projects.teams, projects.environments, projects.environment_connections,
  projects.allocation_rules, projects.allocation_targets TO greenops_worker;
GRANT SELECT, DELETE ON projects.teams, projects.environments, projects.environment_connections,
  projects.allocation_rules, projects.allocation_targets TO greenops_fanout; -- organization erasure (ADR-0027)

-- +goose Down
DROP TABLE projects.allocation_targets, projects.allocation_rules, projects.environment_connections, projects.environments;
ALTER TABLE projects.projects DROP CONSTRAINT projects_team_fk;
ALTER TABLE projects.projects DROP COLUMN team_id;
DROP TABLE projects.teams;
ALTER TABLE cloudaccounts.connections DROP CONSTRAINT connections_tenant_id_id_key;
ALTER TABLE projects.projects DROP CONSTRAINT projects_tenant_id_id_key;
