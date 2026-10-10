-- +goose Up
-- Kubernetes right-sizing recommendations (ADR-0031): smaller CPU and memory requests for workloads that, measured over
-- two weeks, use a fraction of what they ask for.
ALTER TABLE recommendations.recommendations DROP CONSTRAINT recommendations_type_check;
ALTER TABLE recommendations.recommendations
    ADD CONSTRAINT recommendations_type_check CHECK (type IN ('rightsizing','region_shift','time_shift','spot_migration','k8s_rightsizing'));

-- The recommendation job (worker role) reads the measured usage.
GRANT USAGE ON SCHEMA kubernetes TO greenops_worker;
GRANT SELECT ON kubernetes.workload_energy TO greenops_worker;

-- +goose Down
DELETE FROM recommendations.recommendations WHERE type = 'k8s_rightsizing';
ALTER TABLE recommendations.recommendations DROP CONSTRAINT recommendations_type_check;
ALTER TABLE recommendations.recommendations
    ADD CONSTRAINT recommendations_type_check CHECK (type IN ('rightsizing','region_shift','time_shift','spot_migration'));
