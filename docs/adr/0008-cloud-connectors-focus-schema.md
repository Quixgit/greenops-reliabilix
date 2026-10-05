# ADR-0008 Cloud connectors behind one interface; FOCUS as the usage schema

Status: accepted

Decision: CloudProvider is a plugin interface (Validate, GetUsage). AWS is first, through STS AssumeRole with a per-connection ExternalId into a customer-created read-only role; the platform never stores a customer access key. The first source is Cost Explorer (daily, grouped by service and region); CUR/FOCUS exports via S3+Athena come next. Raw provider payloads are archived to object storage and normalized by ingestion into FOCUS v1.4 aligned records (ServiceName, ServiceCategory, RegionId, ConsumedQuantity/Unit, BilledCost, EffectiveCost, BillingCurrency, ChargePeriodStart/End) stored in usage.usage_records. Cost-only sources leave consumed_* NULL. Azure and GCP (Cost Management, BigQuery billing export) are phase 2; check first whether a provider can already export FOCUS natively.
