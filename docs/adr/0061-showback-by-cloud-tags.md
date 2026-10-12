# ADR-0061: Cost by cloud tag (showback)

Status: accepted

## Context
"Who spends what" is the first thing a FinOps team asks of a cost platform, and the platform split cost only by project, team and environment rules the customer wrote by hand. The clouds already know:
resources carry tags (labels), and the bills say which. A cost platform without a split by tag is weaker than every FinOps product.

## Decision
**What is stored.** `usage.tagged_cost` (migration 0075): per day, service, currency, tag key and tag value, the billed and effective cost of the charges that carried that value. It is a
view of the same money as `usage.usage_records`, never added to it. Only the organization's **allocation keys** are read and stored (setting `reporting.tag_keys`, default team, env,
environment, project, product, application, app, owner, cost-center, costcenter, department, business-unit; at most 12), so the table grows with the number of distinct values, not of
resources. Keys are normalized (lower case; the `user_`/`user:` prefix some clouds add is removed); tags a cloud sets itself (`aws:`, `goog-`, `microsoft.`) are never allocation.

**How each cloud is read** (providers aggregate, ingestion stores through the optional `TagNormalizer` and `TaggedStore` interfaces; tags never fail a sync):
- **Azure, Cost Management API** (the usual connection): one query per key and window, grouped by service and the tag key, billed cost; values are lower case (Azure compares them so).
- **Azure, FOCUS export** and **AWS, FOCUS data export**: the `Tags` column of each row, in the same pass that reads the cost, folded into the aggregate (`focus.TagSums`). AWS values keep their case.
- **Google Cloud, BigQuery export**: a second query over the `labels` of the usage rows, for the allocation keys only (the keys are literals that passed the same pattern as the setting);
  effective cost is cost plus credits. An export without a `labels` column leaves tags out.
- **AWS through Cost Explorer only** has no tags: Cost Explorer would need a query per key and a cost per page; the FOCUS export is the supported way.

**Window.** What a source reports for a day replaces what was stored for it (`ReplaceTagged`), so a tag that is gone from the bill is gone here. Tags of a separate query (Azure API, Google Cloud) are read
for at least the last 31 days on every sync, so a month's showback is not mostly "untagged"; the AWS export carries tags only for the days its sync reads. `tags_from` in the answer says since when tags are stored.

**API and screen.** `GET /finops/tag-keys` (keys with how much of the bill each explains) and `GET /finops/showback?tag=` (value, cost, share, three biggest services; untagged and coverage) in the
reporting currency; Projects > Showback. Cost without the key is **untagged and never spread** over the values; the coverage is shown next to every key, because a split that explains 8% of the bill
must not be read as the whole story.

## Not done
Carbon by tag (carbon is computed per service and region, not per resource; an allocation by cost share would be an invented figure); merging spellings of one meaning (env and environment are two keys);
shared-cost allocation rules by tag (the existing team rules stay); tag backfill beyond the sync windows; AWS Cost Explorer tags. Verified live on the stand's Azure subscription (real tags read, totals equal to
the usage rows); AWS and Google Cloud are tested against fixtures and fake services only, not live accounts.
