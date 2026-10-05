# ADR-0009 OpenTelemetry

Status: accepted

Decision: OTel traces/metrics/logs -> Prometheus, Tempo, Loki, Grafana. Every request carries request_id/trace_id/tenant_id; business metrics (sync duration, records processed, calculations) alongside system metrics.
