# ADR-0005 Go 1.27, stdlib net/http, gRPC internally

Status: accepted

Decision: ServeMux method patterns instead of a router dependency; REST+OpenAPI externally, gRPC internally, shared bootstrap in pkg/service.
