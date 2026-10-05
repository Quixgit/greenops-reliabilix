# ADR-0001 Modular monolith

Status: accepted

Decision: one Go module, three processes (api, worker, scheduler) plus an admin CLI, 13 domain packages with an identical layer template (domain / application / repository / http + service.go). Not microservices from day one: a network boundary (contracts, versioning, distributed tracing, per-service CI/CD) is paid for only when it buys something measurable (ADR-0005).

Enforcement: golangci depguard forbids any domain importing another domain and forbids transport/persistence/SDK imports in domain/. Domains cooperate only through small ports declared by the consumer and wired in internal/app (composition root). Extraction later moves a package nearly as is.
