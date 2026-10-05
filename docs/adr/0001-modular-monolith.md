# ADR-0001 Modular monolith

Status: accepted

Decision: one Go module, three processes (api, worker, scheduler), 12 domain packages with an identical layer template. Not microservices at the start: a network boundary is paid for only when it buys something measurable (see ADR-0005). Domain boundaries are enforced by lint so extraction stays cheap.
