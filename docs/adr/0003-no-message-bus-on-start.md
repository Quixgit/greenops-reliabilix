# ADR-0003 No message bus on start: job chains on Asynq

Status: accepted

Decision: no NATS/Kafka/Redis Streams event bus and no gRPC between modules. Domain-to-domain flow inside the monolith is a chain of Asynq jobs: sync_aws_account -> carbon:calculate -> recommendations:calculate. Each job carries tenant_id explicitly, is idempotent (upserts keyed by natural keys) and bounded (retries, timeouts); unique jobs deduplicate; permanent failures (access denied, bad payload) skip retries. Redis is used for the queue, never as a source of truth. Revisit when a domain is extracted (ADR-0005).
