-- +goose Up
-- Every CI gate evaluation is kept: which project, what verdict, how much carbon and cost the planned change adds and
-- why. Without it a pipeline's decisions vanish the moment they are answered; with it the console can show how often
-- the gate passes, warns and blocks, and how much carbon it kept out of production. Only the API role writes it.
CREATE TABLE carbon.gate_evaluations (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id       uuid NOT NULL REFERENCES tenants.organizations(id),
    project_id      uuid NOT NULL,
    verdict         text NOT NULL CHECK (verdict IN ('PASS','WARN','BLOCK')),
    carbon_kg_month double precision NOT NULL,
    cost_delta_month double precision NOT NULL,
    reasons         jsonb NOT NULL DEFAULT '[]',
    changes         integer NOT NULL CHECK (changes BETWEEN 1 AND 200),
    regions         text[] NOT NULL DEFAULT '{}',
    thresholds_applied boolean NOT NULL DEFAULT false,
    source          text NOT NULL CHECK (source IN ('api_key','console')),
    ref             text CHECK (ref IS NULL OR char_length(ref) <= 200),
    created_at      timestamptz NOT NULL DEFAULT now(),
    FOREIGN KEY (tenant_id, project_id) REFERENCES projects.projects (tenant_id, id) ON DELETE CASCADE
);
CREATE INDEX ON carbon.gate_evaluations (tenant_id, created_at DESC);
SELECT platform.enable_tenant_rls('carbon.gate_evaluations', 'tenant_id');
GRANT SELECT, INSERT ON carbon.gate_evaluations TO greenops_api;
GRANT SELECT, DELETE ON carbon.gate_evaluations TO greenops_fanout; -- organization erasure (ADR-0027)

-- +goose Down
DROP TABLE carbon.gate_evaluations;
