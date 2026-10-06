# ADR-0016: Read the customer's FOCUS data export for AWS

Status: accepted

## Context
Cost Explorer is the quickest AWS source but aggregates by service and region. AWS Data Exports can deliver the
bill natively in FOCUS (the schema our usage records already follow), with FOCUS service categories and exact
quantities. The prompt asks us to check whether a cloud already exports FOCUS before writing a parser.

## Decision
- A connection may carry a location (`bucket`, `prefix`, `name`, `region`); when set, the provider reads the
  export instead of Cost Explorer (never both: that would duplicate every cost line). Locations are references,
  validated by strict patterns in the API and again by database constraints.
- The customer's role gets `s3:GetObject` below the export prefix only. Manifest entries must stay inside the
  export's own folder, otherwise the whole read is rejected.
- Files are streamed (gzip CSV) with limits on file count, decompressed size and distinct lines, and folded into a
  compact daily aggregate in the provider; the ingestion module normalizes that aggregate to FOCUS records. The
  two sides share only the JSON contract, never code.
- EC2 charges are named like Cost Explorer does (`... - Compute` for running hours, `EC2 - Other`), so measured
  vCPU/memory hours replace exactly the cost line they correspond to (ADR-0014).
- Changing the location resets the sync cursor and is audited.

## Consequences
Only the CSV export format is supported (no Parquet dependency). A month that has not been delivered yet is
skipped; no manifest at all is reported to the customer. Switching a connection's source leaves rows of the
previous source in place for days outside the new backfill window.
