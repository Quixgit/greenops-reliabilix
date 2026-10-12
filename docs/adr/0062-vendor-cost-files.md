# ADR-0062: Costs of SaaS and other vendors from uploaded files

Status: accepted

## Context
A cloud bill is rarely the whole bill: monitoring, data warehouses, source control and the like are paid to vendors that are not AWS, Azure or Google Cloud, and a cost platform that shows only the three clouds
shows a part of the money. Most of those vendors have no cost API worth building a connector for, but every one of them can export a CSV; many export FOCUS.

## Decision
**One import path for any vendor.** `POST /imports/cost?project_id&vendor&currency` takes a CSV file (at most 8 MB and 200,000 rows) and files it under the provider `saas`: cost only, per day, service and currency,
in the region `global`. The route alone may exceed the 1 MiB request limit (`MaxBodyFor` names it). The file is **checked at once**: a file that cannot be used is refused with the reason and the header it found (422);
otherwise the answer is 202 with a preview (rows, services, window, total, the columns it used, warnings) and the file is read in the background through the ordinary ingestion pipeline (`saas/cost_csv`
normalizer, archive, validation, storage), by the worker, so the API role still never writes usage.

**Understanding the file** (`ingestion/normalizers/costcsv`): the separator is detected; the date and cost columns are found by name from the usual spellings (FOCUS first), and a vendor's own name for them
("CREDITS_USED_COST") when exactly one header contains the word, otherwise the file is refused, never guessed at. Service, currency, effective cost and tags are used when present (a `Tags` JSON column, or `tag_*` /
`label_*` columns). Amounts accept a currency sign, thousands separators, a decimal comma in semicolon files and a credit in parentheses; a date outside 2000 to 2100 or a bad amount skips the row and is counted. Monthly
figures are put on the first day of the month and flagged, not spread. A row of nothing is not stored.

**Taking it back.** Every line carries the source `import:<id>`, its tags the same key; `DELETE /imports/{id}` (only a finished import) removes the lines, tags, stored file and record in the background. A later file with the
same vendor, service and days replaces those days (the lines are keyed that way); re-uploading is therefore safe.

**What it is not.** No carbon: a vendor outside the clouds has no region and no energy model, so the carbon engine reads no `saas` usage (a test and a query clause) and the region tables leave it out; nothing is invented.
The vendor name is part of the service name ("Datadog: APM"), so vendors never mix. Costs enter the totals, the services list, forecasts, budgets and showback like any other cost, converted to the reporting currency.
Note that the reporting currency under "auto" is the one most cost is billed in: adding a large file in another currency can change it.

## Not done
Direct connectors for vendors' own cost APIs (Datadog, Snowflake): they need real accounts to verify and add nothing the file does not; scheduled re-upload (the file is pushed by the person or by their own script with
an API key: `POST /imports/cost` works with a key that holds `cloud:write`); XLSX and JSON files; carbon from a vendor's own sustainability figures; splitting a file's currency by day.
