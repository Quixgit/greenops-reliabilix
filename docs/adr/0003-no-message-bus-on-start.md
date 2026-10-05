# ADR-0003 No message bus at the start

Status: accepted

Decision: Asynq on Redis for background jobs; no NATS/Kafka/gRPC in phase 1. Jobs carry tenant_id explicitly. Revisit when a domain is extracted (ADR-0005).
