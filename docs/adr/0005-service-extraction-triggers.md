# ADR-0005 Service extraction triggers

Status: accepted

A domain is extracted into its own service only when at least one holds: (1) observed load on that domain needs separate scaling (evidence: business metrics cloud_sync_duration_seconds, usage_records_processed_total, carbon_calculations_total in deploy/observability); (2) the domain moves to a separate team that needs an independent release cycle; (3) it needs a different security/compliance profile with physical isolation. Every extraction gets its own ADR (docs/adr/00XX-extract-<domain>-service.md) naming the trigger.
