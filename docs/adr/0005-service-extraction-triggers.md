# ADR-0005 Service extraction triggers

Status: accepted

A domain is extracted into its own service only when at least one holds: (1) observed load on that domain needs separate scaling (evidence: business metrics such as cloud_sync_duration_seconds, carbon_calculations_total); (2) it moves to a separate team needing an independent release cycle; (3) it needs a different security/compliance profile with physical isolation. Every extraction gets its own ADR naming the trigger.
