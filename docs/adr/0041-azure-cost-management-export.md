# ADR-0041: Azure connector on the Cost Management export in Blob Storage

Status: accepted · 0.1.0-alpha.53

## Context
Azure is the third cloud of the catalog. Like AWS (FOCUS data export, ADR-0010) and Google Cloud (BigQuery export,
ADR-0018) it must keep two promises: no customer credential is stored, and one tenant can never read another
customer's billing data through the platform's own (legitimately authorized) identity.

Azure offers Cost Management exports in the FOCUS 1.0 format ("Cost and usage details (FOCUS)") written as CSV, plain
or gzip, to a customer-owned Blob Storage container. The Cost Management query API is rate limited, aggregates
server-side and has no invoice-level detail, so it is not used.

## Decision
- **Source:** the customer's FOCUS cost and usage export in Blob Storage. The platform reads the files with its own
  Microsoft Entra application (a multi-tenant application with a client secret kept sealed in the integration store,
  ADR-0029). The customer creates that application's service principal in their tenant (`az ad sp create`) and
  grants it **Storage Blob Data Reader on the one container**. Nothing secret is exchanged; no SAS token or key is
  accepted (a reference containing `?`, a URL or a host is rejected).
- **References:** `account_ref` is the subscription id (a billing-scope export covers many subscriptions; only rows
  whose `SubAccountId` is that subscription are read), `credential_ref` is
  `azblob://<tenant id>@<storage account>/<container>/<export directory>`. Account and container names, the tenant
  GUID and every directory segment are validated by strict patterns; the storage host is built from the validated
  account name, never taken from input. Redirects are never followed, so the token cannot leave the storage host.
- **Ownership proof (confused deputy):** the container must carry the metadata entry `<token with _>: 1`. Only
  someone with write access to the container can set it. The check repeats on every sync.
- **Export layout:** `<directory>/<yyyymmdd>-<yyyymmdd>/<run>/part_*.csv[.gz]`. For every date range that overlaps the
  sync window the files of the newest run are read (a re-delivery writes a new run; the newest is complete). Files
  outside the export directory, other extensions and `manifest.json` are ignored.
- **Bounds:** at most 500 files, 2 GiB per stored file, 8 GiB per decompressed file, 24 GiB per sync, 1 MiB per CSV
  line, 500k distinct aggregate lines; a violation fails the sync instead of truncating.
- **Normalization:** the provider returns a compact daily aggregate (the same idea as the AWS export). Lines that
  Azure bills exactly are measured: virtual machine running hours x the SKU's core count (`x_SkuCoreCount`) become
  vCPU hours filed under "Virtual Machines - Compute", and Blob "Data Stored" GB/Month lines become storage
  GB-months under "Storage - Capacity". Their cost lines carry the same names, so the carbon engine replaces exactly
  those lines (ADR-0014). Windows licence meters (the same hours again), reservation purchases, operations and
  everything else stay cost lines with a cost-based estimate.
- **No independent control total:** like the AWS export, an invoice-level export is its own source of truth, so
  reconciliation reports "unchecked" with that reason.
- **Customer-fixable failures** are specific and never retried: the application has no service principal in the
  tenant or no role (access denied), the container is missing or nothing delivered yet (export not found), the
  metadata entry is missing (ownership not proven), no row of the subscription in any file (wrong subscription or
  export scope).
- **Regions:** Azure `RegionId` values ("westeurope", "eastus2") are mapped to grid zones next to the AWS and Google
  ones (verified by `admin doctor`, correctable with `ELECTRICITYMAPS_ZONE_OVERRIDES`). A candidate region for a move
  is only proposed inside the same cloud.

## What is not done
- **Memory** is not measured: the export carries no memory size per SKU, so a VM figure is vCPU only (an undercount of
  the RAM share, stated in the methodology).
- **Managed disks** are billed per disk, not per GB, so they stay cost lines.
- **EA/MCA multi-currency** is converted at ECB reference rates like every other currency (alpha.51); a currency
  without a rate keeps no cost-based estimate.
- The column names of the FOCUS export (`x_SkuCoreCount`, `x_SkuMeterCategory`, `x_SkuMeterName`,
  `x_SkuMeterSubcategory`) were taken from Microsoft's FOCUS column documentation and are not yet exercised against a
  live export: a missing optional column only means fewer measured lines (more cost-based), never a wrong total.

## Consequences
- A customer needs a storage account and an export before connecting; the setup steps and `deploy/azure/customer-access.tf`
  cover the rest.
- The platform needs `PLATFORM_AZURE_CLIENT_ID` and `PLATFORM_AZURE_CLIENT_SECRET` (console: Platform Microsoft Azure
  application); without them the connector is off and Azure is refused at connect time.
