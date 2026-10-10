-- +goose Up
-- Container usage next to energy (ADR-0030): what a workload used of what it asked for. This is the measured base for
-- rightsizing; the columns are optional because an agent without cAdvisor or kube-state-metrics sends energy only.
ALTER TABLE kubernetes.workload_energy
    ADD COLUMN cpu_core_seconds     numeric(18,3) CHECK (cpu_core_seconds IS NULL OR cpu_core_seconds >= 0),
    ADD COLUMN memory_avg_bytes     numeric(20,0) CHECK (memory_avg_bytes IS NULL OR memory_avg_bytes >= 0),
    ADD COLUMN cpu_request_cores    numeric(12,3) CHECK (cpu_request_cores IS NULL OR cpu_request_cores >= 0),
    ADD COLUMN memory_request_bytes numeric(20,0) CHECK (memory_request_bytes IS NULL OR memory_request_bytes >= 0);

-- +goose Down
ALTER TABLE kubernetes.workload_energy
    DROP COLUMN memory_request_bytes, DROP COLUMN cpu_request_cores, DROP COLUMN memory_avg_bytes, DROP COLUMN cpu_core_seconds;
