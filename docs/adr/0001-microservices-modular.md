# ADR-0001 Microservices with domain modules

Status: accepted

Decision: bounded-context services (6 for MVP), each with domain/application/infrastructure/transport layers. Not one service per table. Consequence: independent scaling/deploy; requires contracts (proto, events) and per-service DBs.
