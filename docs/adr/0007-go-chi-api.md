# ADR-0007 Go 1.27, net/http + Chi, REST + OpenAPI

Status: accepted

Decision: net/http with Chi for routing (not Fiber: the product is domain logic, integrations and workers, not raw HTTP throughput; Chi keeps the standard stack visible). REST + JSON with RFC 9457 problem responses; OpenAPI 3 in backend/api/openapi.yaml is the contract. A unit test fails the build if a registered route is missing from the spec or the spec documents a route that does not exist.
