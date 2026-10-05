# Event catalog

Transport: NATS JetStream. Subject pattern: `greenops.<domain>.<event>.v<major>`.
Envelope: see `pkg/events` (`id`, `subject`, `tenant_id`, `occurred_at`, `trace_id`, `data`).
Rules: consumers are idempotent on `id`; breaking changes create a new `v<major>` subject.

| Subject | Producer | Consumers | Data |
|---|---|---|---|
| `greenops.cloud.sync_completed.v1` | cloud-integration | ingestion | `account_id`, `from`, `to`, `raw_object_key` (S3) |
| `greenops.usage.normalized.v1` | ingestion | carbon, finops | `project_id`, `period`, `record_count` |
| `greenops.carbon.calculated.v1` | carbon | recommendation, reporting | `project_id`, `period`, `co2e_kg`, `methodology_version` |
| `greenops.recommendation.created.v1` | recommendation | reporting, notifications | `recommendation_id`, `type` |
