# Changelog

All notable changes. Format: [Keep a Changelog](https://keepachangelog.com), versions as in
[`docs/release/versioning.md`](docs/release/versioning.md). The version of every part is in [`VERSIONS.md`](VERSIONS.md).

## [Unreleased]

## [0.1.0-alpha.86] - 2026-10-11

### Fixed
- The delete confirmation fits inside the project card: a compact layout (title, a two-line note, the two buttons) that
  never overflows the card's lower edge, with the note replaced by the reason when the server refuses.

## [0.1.0-alpha.85] - 2026-10-11

### Fixed
- The delete confirmation on a project card: Cancel and Delete are two equal buttons kept clear of the card's edges.

## [0.1.0-alpha.84] - 2026-10-11

### Added
- **Delete a project.** `DELETE /api/v1/projects/{id}` removes an empty project (its policy, units, budgets and
  allocation settings go with it); a project that still has cloud connections or measured data is refused with 409.
  On the Projects page a round delete button pops out of a card's corner on hover, the confirmation wipes over the
  card and the card dissolves when it is deleted.

## [0.1.0-alpha.83] - 2026-10-11

### Changed
- **New project wizard opens inside the Projects page**: it slides out above the summary and the cards stay where they
  are; a labelled stepper (name, unit, team, review) with a progress line, in the platform's card style.
- **Overview KPI cards**: the sparkline area and the footer have a fixed height, so the footer text of the four cards sits
  on one level.
- **Connected accounts is a list**: one row per platform with its own drawn mark (AWS, Azure, Google Cloud,
  Kubernetes, each in its own soft colours), the state with the last sync and a Connect / Open action.

## [0.1.0-alpha.82] - 2026-10-11

### Changed
- **Primary buttons and the user menu are flat**: one solid green with no gradient, sheen or shadow (the previous
  release made them raised by mistake); the user menu is a plain avatar and name that gets a light fill on hover.

## [0.1.0-alpha.81] - 2026-10-11

### Changed
- **Primary buttons are no longer flat**: a soft green gradient with a light top edge, depth, a sheen that sweeps over
  on hover and a press state. Used on the sign-in button, the wizard, projects, settings and forms.
- **The user menu in the header** is a bordered pill with a raised avatar and the same hover and open states as the
  other header controls.

## [0.1.0-alpha.80] - 2026-10-11

### Changed
- **Sign-in page promises** have their own drawn pictures (cost and carbon, grid measurement, insight to action,
  trust): larger, with a second tone, no white tile or frame, gently floating.

## [0.1.0-alpha.79] - 2026-10-11

### Changed
- **New project is a step-by-step wizard in the page itself**, not a side panel: name, functional unit (with presets),
  team, review, then what to do next (connect a cloud, choose regions, report units). Animated steps and progress.

## [0.1.0-alpha.78] - 2026-10-11

### Changed
- **The rest of the stock icons are gone**: the operator console, sign-in, pagination, map controls, menus and forms use
  the platform's own drawn icons (about forty new ones: server, network, eye, pencil, trash, card, power, calculator…).
- **Dates in the operator console** (plan expiry, grid-data backfill) use the platform calendar.
- **Info tooltips (ⓘ)** on the remaining panels: all settings tabs, alerts, carbon, Kubernetes, methodology, reports,
  cloud account, teams and allocation; English and Russian.

## [0.1.0-alpha.77] - 2026-10-11

### Changed
- **Projects page rebuilt.** Summary tiles (projects, cost in the period, projects with allowed regions, budgets needing
  attention), a card per project with its cost and share, clouds, allowed regions, CI gate and budget use, tabs for
  teams and allocation and for budgets, a "New project" panel, and a detail panel with structure, cloud connections,
  policy and a work-units form that reports units with the new calendar.
- **Top header redone**: search (Ctrl/Cmd+K), period presets with a calendar for a custom range, project, language,
  alerts list in the bell and the user menu; the page no longer jumps when the bell opens (menus are not modal).
- **Calendars** in the platform's style (animated month, soft range band) for the period and for the report dates.
- **Form fields** show a light leaf edge on hover and focus instead of the browser's blue outline; **vertical
  scrollbars** are thin and fade in.
- **Connected accounts** block redone with one icon set for AWS, Azure, Google Cloud and Kubernetes; the small map's
  window is compact; window close crosses are small circles.
- **Icons** in the KPI strip, FinOps tiles, setup guide and header are drawn larger and bare, without a tile or frame.

### Added
- **"Remember me"** on the sign-in page keeps the user name or e-mail on the device (never the password).

## [0.1.0-alpha.76] - 2026-10-11

### Changed
- **One green and one set of soft tones for the whole platform.** Charts, bars, tiles, switches, buttons, the phoenix, the
  menu card and the sign-in scene now share a single green (`leaf`, with a soft and a deep shade) and four pastel tones
  (blue, amber, rose, violet), defined once as theme colours; the several greens and saturated bar colours that had
  grown in different places are gone.
- **The map**: clicking a point opens a small window anchored to it (city, country, cloud, region id, grid intensity
  with its band, and your CO₂e and cost there) with a close cross; the enlarged map's zoom and navigation buttons work
  again (the map captured the pointer and swallowed their clicks) and gain pan arrows; the region list is one line per
  place with its clouds (Paris was listed three times, once per cloud) and the "cleanest grids" chips likewise.
- **Close controls** of windows and panels are one drawn cross in a soft square that turns on hover.

## [0.1.0-alpha.75] - 2026-10-11

### Fixed
- **A region move is only proposed inside the workload's own cloud.** The generator picked the cleanest allowed region
  whatever its cloud, so an Azure workload could be told to move to an AWS region. A region's cloud is read from its
  shape (`eu-central-1` AWS, `europe-west9` GCP, `westeurope` Azure) and other clouds' regions are skipped.

### Added
- **"Where may this project run?"** On the Overview, the empty recommendations, the optimization card and the regions
  panel open a panel that lists the regions in use (selected) and cleaner regions of the same cloud with how much
  cleaner they are; saving writes the project's allowed regions (the rest of the policy is kept) and queues a new
  analysis. Recommendations needed exactly this and offered only a link to a text field before.
- **ⓘ on every figure and block** of the Overview, Connectors and Organization: what it is and how it works, on
  hover, focus or tap, drawn above the page so no card clips it.
- **The platform's own icon set** (about forty hand-drawn icons, one pen and a soft second tone) in the menu, the
  cards and the sign-in page, instead of stock glyphs.

### Changed
- **Regions on the Overview**: the map selects a region and a panel answers for it (grid now and over time, footprint
  there, what a move would change, why no comparison is possible and how to enable it); the footprint ranking and the
  cleanest grids sit under the map, so there is no empty space.
- The carbon series on the trend chart is filled in green like the cost series is in blue.
- **Sign-in** is light: a pale scene with the same dot-matrix world, regions and light travelling along arcs, and a
  form with Google and GitHub, e-mail, "forgot your password" and "create an account" that slide into each other.

## [0.1.0-alpha.74] - 2026-10-11

### Changed
- **The sign-in page is new**: a dark scene with the dashboard's dot-matrix world held low and alive (cloud regions
  pulse, light travels between them along arcs, soft aurora behind), the phoenix and a headline for GreenOps and
  FinOps on the left, a frosted form on the right. It stands still with reduced motion. The form, the forgot-password
  flow and the identity-provider hand-off are unchanged; passwords and second factors are still typed only on the
  provider's page.
- **Overview: regions and the right column are interactive.** Each used region is a card with its grid intensity, paired
  CO₂e and cost bars and, when no comparison is possible because no regions are allowed, a link that says so and
  goes to the policy. Connected-accounts tiles lead to the connect panel of that cloud; recent events lead to the
  screen that holds them.

## [0.1.0-alpha.73] - 2026-10-11

### Changed
- **The Overview is rebuilt for GreenOps and FinOps together.**
  - Four KPI cards with a trend line each: cloud cost (run-rate per day, last 7 days against the 7 before), operational
    CO₂e, carbon intensity (with the cleanest grid now) and optimization potential (with the reason when there is none).
  - A FinOps strip: **month-end forecast** (spent so far plus the 7-day run-rate), **budget burn** with projection,
    **cost anomalies** and **grams of CO₂e per unit of currency spent**.
  - Cost and carbon over time on two correct axes, with a crosshair, spikes marked and a 7-day average.
  - Where it comes from, by cloud or by region, as paired cost and CO₂e bars with the grid intensity of each region.
  - Services as one ranked board (share of cost and of emissions side by side, carbon-heavy services flagged, trend,
    paging), replacing the bubble chart and the table.
  - "Get the full picture": a checklist that says why a block may be empty and what unlocks it (connect, first sync,
    allowed regions, work units, budget, notifications); it disappears when everything is set up.
  - Data quality now also says whether cost matches the cloud's totals and explains estimates when grid readings are
    not day averages. Empty recommendations and efficiency blocks give the reason and a way forward.
- **Brand:** the phoenix of Reliabilix Systems in green and alive (a light band travels across the plumage, the glow
  breathes), "Reliabilix Systems" in the menu, and the card at the foot of the menu rotates its message (the tagline,
  the three verbs, your impact, and the cleanest grid right now).

## [0.1.0-alpha.72] - 2026-10-11

### Added
- **The Organization tab works end to end.** Monthly budget, carbon budget and the alert threshold open in place and
  save (the budget creates or replaces the organization's monthly budget); a cloud that is not connected links to the
  connect panel of that cloud (`/connectors?connect=aws`), Kubernetes to the agent setup.
- **Real grid data sources** (`GET /dashboard/grid-sources`): the sources the platform has readings from, how many
  regions each covers, how fresh it is and what share of the organization's own figures it supplied. Replaces a fixed
  list that named sources the platform does not use (WattTime stays, marked planned).

### Changed
- Automation rows say what is true: region moves and rightsizing always need a person; **time shifting** is the one
  type a project may let the platform apply on its own, counted per project (it was shown as "rightsizing").
- Security shows the invitation lifetime and the allowed e-mail domains (settings that exist) and says plainly that
  two-factor authentication and session length belong to the sign-in provider; roles show built-in and custom counts.

## [0.1.0-alpha.71] - 2026-10-11

### Changed
- **Data & integrations is rebuilt**: six cards of one size (data sources, cost reporting, alerts, access policy,
  export, erasure) with compact rows and one save bar, instead of every settings group dumped as a form. Gone from it:
  the search and the group tabs; the modules and quota fields (the plan decides them and the client cannot change them;
  usage and modules are on Billing); the suspended-organization form (a notice shows only when it applies).
- **Notifications have one meaning per place.** The organization-wide delivery switch lives only on the Notifications
  tab; each destination keeps its own pause there. The Organization tab and the Connectors page show the state
  (on, paused, switched off) without a second switch, and a channel no longer reads "on" while delivery is switched off.
- Switches and checkboxes use a calm teal instead of the bright mint; cards in a row have the same height.

## [0.1.0-alpha.70] - 2026-10-11

### Added
- **Rename the organization** (`PATCH /tenant`, administrators; the name is 2-100 characters; audited as
  `organization.renamed`). The Organization tab saves the name together with the reporting currency. Migration 0052: the
  change is a database function, because the api role has no UPDATE on organizations.
- **Audit log in the console** (API & security): who did what, filterable by area (members, connections, API keys,
  recommendations, settings, organization).

- **Paging wherever a list can grow**: one paging bar (range "11–20 of 57", page size 10/25/50, previous and next,
  numbers with gaps collapsed) on the audit log, change history, deliveries, members, API keys, project policies,
  recommendations, alerts, reports, Kubernetes workloads and a connection's sync history. It hides when one page is enough.

### Changed
- **Every Settings tab follows the Organization layout** (cards with an outlined icon, a purpose line, rows and hover):
  Users (members with initials, role and removal, invitations, pending invitations; custom and built-in roles),
  Data & integrations (every settings group is a card; export and erasure), Cost & carbon policies (projects and their
  policies, budgets), Automation, Notifications (destinations, add, deliveries), API & security (keys, audit log, change
  history), Billing (plan, usage, change) and Preferences.

## [0.1.0-alpha.69] - 2026-10-11

### Changed
- **Settings follows the design: tabs on top and a card layout.** The left admin navigation is gone; the tabs are
  Organization, Users, Data & integrations, Cost & carbon policies, Automation, Notifications, API & security, Billing and
  Preferences (the old addresses such as `#members`, `#keys`, `#projects`, `#profile` land on the right tab).
  **Organization** is the design's grid, from real data: organization details (the reporting currency is saved here),
  cloud provider connections with their state and last sync, carbon and energy data sources, cost and carbon policy
  (budget, carbon budget, alert thresholds, projects with allowed regions), automation and CI gate, notification
  channels with working switches (they enable or disable the destinations of that kind) and security and access.
  Rows that need a person or the sign-in provider say so instead of showing a switch that does nothing. Automation and
  Preferences are cards of the same look; every other tab keeps its panels in the same card style. Reduced motion is
  respected; translated (English, Russian).

## [0.1.0-alpha.68] - 2026-10-11

### Changed
- **The connection page says what the connection is and what to do** (it showed neither before): the project it belongs
  to, how the platform authenticates (IAM role, access key, service account key, service principal, shared dataset,
  export), the cost and days with data of the period, and, from the API's own setup text, the steps to set up on the
  customer's side. A failed "Test connection" is no longer a dead end.
- **A connection can be disconnected from its page** (two clicks). Its cost history stays; connecting the same account
  again updates those lines.
- The empty Recommendations page says why it is empty beyond "not synced yet": a move to a cleaner region is only
  proposed into regions the project's policy allows, and links to where they are set.
- Gap list: stand-wide check after removing the demo data (all customer read endpoints answer, empty states are honest).

## [0.1.0-alpha.67] - 2026-10-11

### Fixed
- **Two accounts of one provider no longer overwrite each other's costs.** A usage line was keyed by provider, service,
  region and day, so a second AWS account (or Azure subscription, or GCP project) in the same project replaced the first
  one's lines on every sync. The account is part of the key now (migration 0051, existing lines get their account from
  their connection). It is the account, not the connection, on purpose: deleting a connection keeps its history, and
  connecting the same account again updates those lines instead of doubling them. Covered by an integration test.

### Added
- **Cost of every connection** (`GET /dashboard/connections`: billed cost in the organization's currency, days with data
  and the newest day). The Connectors page shows it per connection; CO₂e is shown only where one connection is the whole
  provider (carbon is stored per provider and region, a split would be invented).
- The Electricity Maps tile on the Connectors page shows the live state of the grid data (fresh readings, or waiting when
  the newest is older than a day) and when it was updated.

## [0.1.0-alpha.66] - 2026-10-11

### Changed
- **The Connectors page is redesigned** around what is really connected: four indicators (active connectors, source
  types, sync health from the latest runs, average sync time), one list of connections with status, last sync, a bar per
  recent run, and cost and CO₂e of the period; a row opens to project, access mode, data check, last error and "Sync
  now". A "needs attention" panel lists broken connections with their cause and a way to the fix, the catalog below
  shows each connector's real state, and "Connect" opens a slide-over instead of an inline form. Entrance, filter,
  row-expand and slide-over animations respect reduced motion.
- **One source of truth for connector state** (`lib/connectors.ts`): the Overview tile and the Connectors page derive
  it from the same facts. Kubernetes is no longer shown as "Soon" (it is a real connector, ADR-0030): it reads as
  connected while a cluster pushes data and as waiting when the data is older than two days; e-mail, Slack and webhook
  read as connected while a destination is enabled.
- The page and its catalog are translated (English, Russian).

### Changed
- The stand's scheduler unit no longer switches off `cloudaccounts:sync_all` and `cloudaccounts:rightsizing_all`: the
  stand holds a real Azure connection now, and with the switch the data would never refresh by itself.
  `docs/setup/stand.md` says not to load the demo dataset next to real connections and how to remove it (including the
  stale per-unit figures it leaves in carbon calculations until they are recalculated).

## [0.1.0-alpha.65] - 2026-10-11

### Added
- **AWS and GCP connect with the customer's own read-only credential, like Azure** (ADR-0044). Until now both asked the
  customer to build something (a role with an ExternalId; a billing export, a dataset label and a shared dataset).
  - **AWS: an IAM user access key** (`awskey://<access key id>` + the secret access key). Cost Explorer, the S3 FOCUS
    export and rightsizing are read with it directly; only long-term `AKIA…` keys are accepted.
  - **GCP: a service account key** (the JSON key file). Queries run with the customer's service account in its own
    project, no ownership label and no Google identity of the platform needed: the GCP connector is available on every
    platform, `PLATFORM_GCP_PROJECT` is only for the advanced shared-dataset mode.
  - The secret is sealed exactly like the Azure client secret (AES-256-GCM bound to the organization and the connection,
    write-only, never in a response, a listing or the export). The key file is accepted only if it is a service account
    key with Google's own token URI; a user credentials file is refused.
  - The connection form offers the own credential as the recommended way for every provider and keeps the older way
    (role, shared dataset, export) as "advanced". A connection reports `access` (`role`, `access_key`,
    `dataset_share`, `service_account_key`, `service_principal`, `export`).
- `admin aws-check` and `admin gcp-check` accept the customer's own credential from the environment
  (`CLOUD_CHECK_SECRET`, `CLOUD_CHECK_KEY_FILE`).

### Changed
- API: `POST /cloud-accounts` takes `client_secret` for an AWS `awskey://` connection and, optionally, for GCP; a
  secret on any other kind is refused (422). `access` added to the connection.

## [0.1.0-alpha.64] - 2026-10-11

### Fixed
- **AWS and GCP failures now say why**, as Azure's do since alpha.60. Until now everything that was not "access denied"
  showed only "Sync failed; it will be retried automatically".
  - AWS: a refused AssumeRole (check the role's trust policy and ExternalId), Cost Explorer not enabled for the account,
    a missing permission, throttling, "no data yet", and any other API error with AWS's error code. When the platform's
    own AWS identity has no usable credentials the message says so instead of blaming the account.
  - GCP: BigQuery throttling and 5xx (with the status), an export table that lacks the standard columns, the per-query
    byte limit, and any other status with Google's reason code.
- **GCP: throttled or briefly failing BigQuery requests are retried** (429 and 5xx, up to four attempts, honouring
  `Retry-After`, at most a minute) instead of failing the sync at the first answer. AWS: the SDK retries up to six
  attempts for Cost Explorer throttling.
- **AWS: regions that map to "global" are merged.** Cost Explorer's "NoRegion", "global" and "aws global" were stored as
  separate rows of one key and overwrote each other, losing cost (the same defect Azure had in alpha.60).

### Added
- `admin aws-check <account id> <role arn> <external id>` and `admin gcp-check <project id> <bq://project/dataset/table>
  <external id>`: verify, read the last seven days and read the control totals of one connection with this machine's
  platform identity, printing the exact text the customer would see on failure and whether it needs their action.
  They need neither the database nor the platform configuration.

## [0.1.0-alpha.63] - 2026-10-10

### Fixed
- **A sync failed for good when the database already held rows of a month that had no partition.** PostgreSQL refuses to
  create a monthly partition while the DEFAULT partition holds rows of that month ("updated partition constraint for
  default partition ... would be violated"), so every sync that touched the month (any backfill reaching back into it)
  failed on every retry, with only "Sync failed; it will be retried automatically" to show for it. Rows land in the
  default partition when data of a month arrives before its partition exists (a seed, a manual load). The function that
  creates partitions now moves such rows into the new partition in the same transaction. Migration 0050; covered by an
  integration test that reproduces the exact error before the change.
- Found by running the Azure connection against a real subscription: the cost read itself (1203 records over 60 days,
  all valid, totals reconciled) was never the problem.

## [0.1.0-alpha.62] - 2026-10-10

### Fixed
- **A connection that never delivered data no longer looks connected.** The Overview tile, the Connectors list and the
  connection page said "Connected" / "healthy" as soon as the access check passed, even when the first sync had failed
  or not run: the sign that nothing was being read was a "never" in a corner. They now say "Waiting for first sync",
  "Sync failing" or "Needs attention", and "Connected" only after a successful sync.
- **A temporary sync failure is shown on the connection** (Last error) instead of "none": it keeps the connection's state
  (the next attempt may work) and clears itself on the first success.
- **Azure and throttling:** Cost Management's own retry headers (`x-ms-ratelimit-microsoft.costmanagement-*-retry-after`)
  are honoured, the longest wait wins, up to two minutes and five attempts, and queries of one sync are paced two
  seconds apart.

## [0.1.0-alpha.61] - 2026-10-10

### Added
- **`admin azure-check`**: signs in as an Azure service principal and runs the three queries of a sync (billed cost per
  day, billed and amortized cost by service and location), printing the status, the columns and the first rows or
  Azure's own error. `AZURE_CLIENT_SECRET='...' admin azure-check <subscription> <tenant> <application>`; it needs no
  database or platform configuration and prints no secret.
- An end-to-end test of the Azure path (API seals the secret, a separate worker container opens it, costs are read from a
  Cost Management endpoint, stored, reconciled and listed by the dashboard).

### Changed
- The sign-in failure of an Azure service principal says what to check (tenant id, application id, secret value, expiry).

## [0.1.0-alpha.60] - 2026-10-10

### Fixed
- **A failed sync now says why.** Until now every failure that was not "access denied" showed only "Sync failed; it will
  be retried automatically". Azure failures that are safe to show now carry their cause: throttling by Cost Management
  (with the wait), an HTTP status and Azure's own error code, a refused cost query, and, importantly, a stored client
  secret that the service cannot open because the worker runs without the same `NOTIFICATIONS_SECRET_KEY` as the API.
- **Azure: the amortized view is optional.** Some subscription kinds refuse the amortized cost query (HTTP 400); the
  billed amount stands in for it instead of failing the whole sync.
- **Azure: locations that map to one region are merged** ("", "Unassigned", "Zone 1" are all "global"): they were stored
  as separate rows that overwrote each other.

## [0.1.0-alpha.59] - 2026-10-10

### Added
- **Azure by service principal** (ADR-0043): the usual way to connect Azure is now the customer's own service principal
  with the **Cost Management Reader** role on the subscription. They enter the subscription id, the tenant id, the
  application id and the client secret; no export, storage account or platform application is needed, and the platform
  reads the costs of every service through the Cost Management Query API (billed and amortized cost per day, service,
  location and currency, with throttling waited out and an independent day total for reconciliation). Cost-based carbon
  estimates are used (the API has no quantities); the export mode stays for measured VM hours.
- The client secret is the one customer secret the platform keeps: sealed (AES-256-GCM, bound to the organization and the
  connection), write-only in the API, absent from every response and from the data export. Migration 0049
  (`cloudaccounts.connections.sealed_secret`), API contract 0.30.0 (`client_secret`, `azcost://` reference).
- Azure is allowed by default for new organizations (the operator can still narrow the list per organization).

### Changed
- The platform's Microsoft Entra application (operator console) is now optional: it is only needed to read a customer's
  Blob Storage export.

## [0.1.0-alpha.58] - 2026-10-10

### Fixed
- **Saving from the browser answered "Method Not Allowed"** for everything that uses HTTP PUT: the operator console's
  integration fields (the Azure application, SMTP, Electricity Maps and the other keys), project policies, operator
  access and, since alpha.57, a project's team and an environment's connections. The web server's proxy to the API
  exported no PUT handler (the API itself always accepted it). The proxy now passes PUT, and a test pins every method
  the screens use.

## [0.1.0-alpha.57] - 2026-10-10

### Added
- **Organization structure** (Projects, ADR-0042): **teams** that own projects, **environments** of a project (production,
  staging, development, test, other) that hold the cloud connections running in them, and **allocation rules** that
  split what one project is billed for (everything, or one service) between projects by weight, the source keeping a
  share when it lists itself. A new report, "Cost by project, team and environment", shows direct cost, cost received
  and handed over, and the total per project, with team and environment roll-ups. Rules are applied when the report is
  read (nothing is rewritten), the totals always add up to the billed cost, a service rule wins over the rule for
  everything, and an amount without a reference rate is left out and counted. Migration 0048, API contract 0.29.0
  (`/teams`, `/environments`, `/allocation-rules`, `/allocation`, `team_id` on a project), data export and erasure
  include the new tables. English and Russian.

### Security
- Every reference of the new tables to a project, team, environment or connection is a composite
  `(tenant_id, id)` foreign key: a plain foreign key is checked without row-level security and would have accepted the
  id of another organization's project. Covered by an integration test.

### Fixed
- Integration tests that still expected the pre-alpha.35 methodology, no currency conversion and a row-level-security
  rule that did not know the global `finops.fx_rates` table now match the product.

## [0.1.0-alpha.56] - 2026-10-10

### Added
- **Budgets in the interface** (Projects): the budgets API existed since the first release and had no screen. The page
  now lists the organization's and every project's budgets with the spend of the current period, the projection for it
  and the state (on track, at risk, exceeded), and lets owners, admins and the billing role add and delete them (scope:
  whole organization or one project; amount, currency, monthly / quarterly / yearly). Budgets are measured in their own
  currency. The section is absent for a role without billing access. English and Russian.

## [0.1.0-alpha.55] - 2026-10-10

### Added
- **Automation page**: the plans of approved recommendations in one place. "To do" and "Finished" lists; a plan shows its
  steps, commands, Terraform, risk level with the reasons, the rollback and who did what and when. Approve (owners and
  engineers), withdraw, and report the result (applied, failed, or applied and rolled back; a note is required for the
  last two, and an applied result marks the recommendation applied). A plan is created from an approved recommendation
  with "Create the plan". The platform still never changes the customer's cloud (ADR-0017). Frontend only, English and
  Russian.

## [0.1.0-alpha.54] - 2026-10-10

### Added
- **SCI calculator** (Carbon & Energy): the person names the functional unit (API request, order, active user) and how many
  were delivered in the period; the page returns SCI = ((E x I) + M) / R in CO2e per unit from the period's own energy
  and carbon, with E, I, M and R shown, an optional comparison with the previous period (own R) and a plain statement
  of inputs and limits (cloud boundary, whether embodied emissions are included, average not marginal intensity, R is the
  person's). Frontend only; the formula is tested in `frontend/lib/sci.ts`. English and Russian.

## [0.1.0-alpha.53] - 2026-10-10

### Added
- **Azure connector** (ADR-0041): reads the customer's Cost Management export (FOCUS 1.0 cost and usage, CSV or gzip)
  from Blob Storage with the platform's own multi-tenant Microsoft Entra application. The customer creates the
  application's service principal in their tenant, grants it Storage Blob Data Reader on the one export container and
  marks the container with the connection's token as metadata (proof of control, re-checked on every sync). No key,
  SAS token or secret of the customer is accepted or stored. A billing-scope export is filtered to the connection's
  subscription. Virtual machine running hours (x the SKU's core count) become measured vCPU hours and Blob "Data
  Stored" becomes measured GB-months; licences, purchases, operations and the rest keep a cost-based estimate. Memory
  and managed disks are not measured (stated in the ADR). Setup steps and `deploy/azure/customer-access.tf` are given
  to the customer; the platform application is entered in the operator console (Platform Microsoft Azure application,
  `PLATFORM_AZURE_CLIENT_ID` / `PLATFORM_AZURE_CLIENT_SECRET`); without it the connector is off.
- 56 Azure regions mapped to grid zones and to places on the map; a region move is only proposed inside the same cloud
  (an Azure id such as `eastus2` no longer fits the Google Cloud shape).
- Operator console: the "Azure Cost Management" stub became the "Microsoft Azure" data source with its connections.

### Changed
- **Cost-based carbon in other currencies**: the carbon engine used to skip every cost line that was not in USD. Cost
  is now converted to USD at the rate of its day (the ECB rates of alpha.51) before the estimate, so a bill in euro or
  pound gets one; an amount in a currency without a rate is still skipped and counted as such. This also applies to
  AWS and Google Cloud accounts billed in another currency: run "Recalculate carbon" to fill earlier days.
- The `providers_allowed` note no longer says Azure is unavailable. API contract 0.28.0 (`azure` in the connection input).

## [0.1.0-alpha.52] - 2026-10-10

### Added
- **Reporting currency**: an organization setting (Settings, Organization, Cost reporting: auto or one of 16 currencies)
  says which currency every cost figure is shown in; "auto" is the currency most of the cost is billed in. Charts, tables
  and totals follow it (symbols and the chart legend too). `finops.reporting_currency` (migration 0047).
- **Nothing silently short**: amounts in a currency without a reference rate are left out of the totals and the cost KPI
  says which currency and how many amounts. `excluded` in the cost summary. API contract 0.27.0.

### Fixed
- A cost sum whose amounts all lack a rate no longer breaks the endpoint (null-safe sums).

## [0.1.0-alpha.51] - 2026-10-10

### Added
- **Currency conversion**: cost in several currencies is now converted into one instead of dropping everything but the
  dominant currency. A daily job loads the European Central Bank euro reference rates (free, no key; the first run loads
  the history); `finops.convert_cost` converts at the rate of the amount's day, carrying a rate over weekends and holidays
  (at most 10 days). Totals, trend, per-service and per-region cost, anomalies, unit cost, verification and budgets use it.
  An amount in a currency the ECB does not publish, or without a rate, is left out rather than guessed. Daily reference
  rates, not transaction rates: for reporting, not for invoicing. Migration 0046, task `finops:refresh_fx`,
  `admin refresh-fx`.

### Changed
- Budgets are measured in the budget's own currency (other currencies are converted into it).

## [0.1.0-alpha.50] - 2026-10-10

### Added
- **Price of a region move (EC2 compute)**: a weekly job reads the on-demand EC2 price list AWS publishes for its pricing
  pages and stores a relative price index per region (geometric mean over a fixed basket of m5, c5, r5, m6i, c6i and t3
  instances, Linux, shared tenancy; baseline us-east-1 = 1.0; 26 regions). Region-shift recommendations and the map's
  relocation layer use it to estimate the cost change of the EC2 compute in the same period as the carbon effect. It is a
  list-price estimate: discounts, savings plans, data transfer and the cost of the move are outside it, and it is applied
  to EC2 compute only (never to storage, databases or functions). `finops.region_price_index`, migration 0045, task
  `recommendations:refresh_price_index`, `admin refresh-prices`. API contract 0.26.0 (`cost_delta`, `cost_basis`).

### Changed
- A recommendation whose cost comes from the index says so ("list-price estimate") with what it excludes in the tooltip;
  other services keep "cost: awaiting price data". Data residency is no longer listed as unchecked.

## [0.1.0-alpha.49] - 2026-10-10

### Added
- **Customer settings rebuilt** (Settings): a section rail (Account, Organization, Access, Billing, Data) with search and
  addressable sections instead of one row of tabs; a profile card with the role and what it can do; **Preferences**
  (default period, carbon unit kg/t/auto, density, reduced motion; kept in the browser); **Projects & policies**: every
  project's allowed regions, what it applies on its own, carbon limit and CI gate in one table, a row opens the policy.
- **Distance and round-trip estimate** for a proposed region move (from the distance between the regions; labelled as an
  estimate), next to the residency rule that is already enforced.

### Changed
- Operator console: Errors, Jobs & scheduler (queues, failed tasks, schedules), Audit log and Access are sortable tables
  with status pills, like the Overview and Organizations.

## [0.1.0-alpha.48] - 2026-10-10

### Added
- **Apply without approval, per project**: the project policy lists the recommendation types the platform may apply on its
  own (`auto_apply_types`; only time shifting can be listed, region moves and production changes always need approval,
  enforced in the domain). The Overview counts "safe to automate" from these policies instead of guessing by type.
  Migration 0044. API contract 0.25.0.
- **Relocation respects data residency**: a cleaner region is proposed only among the regions every project using the
  source region may use (the same rule as the recommendation engine); with no allow-list it says why
  (`relocation_blocked`) and what to set. Residency is no longer in the list of checks still to do.
- **Operator console, redesigned Overview**: eight metrics, platform controls as switches (maintenance mode with a
  confirmation, sign-up, automatic loading of the past), and sortable tables for organizations, queues, paid API usage
  against the daily cap, data sources, plans and integrations. Shared components: switch, checkbox, status pill, meter and
  a sortable data table.

### Changed
- Platform and organization settings use switches for on/off values and checkbox chips for lists; the plan catalog is a
  matrix; the organizations list is a sortable table.

## [0.1.0-alpha.47] - 2026-10-10

### Fixed
- A monthly national figure no longer stops the automatic loading of a region's past when a free hourly source (UK
  Carbon Intensity, EIA, Energy-Charts) covers that region: coverage by hourly readings is checked separately
  (`carbon.region_hourly_coverage`, migration 0043).

## [0.1.0-alpha.46] - 2026-10-10

### Added
- **Energy-Charts as a free source for Europe** (Fraunhofer ISE; no key, CC BY 4.0, data of the transmission system
  operators): hourly intensity COMPUTED from public generation by production type, with history, for DE, FR, ES, IT, PL,
  SE, FI, BE, NL, CH, AT, NO, DK, PT, CZ, HU, GR, IE. It fills the past of European regions without a paid plan. Source id
  `energy_charts`, integration `energy_charts` (address only), default order Electricity Maps, UK Carbon Intensity, EIA,
  Energy-Charts, Ember. Migration 0042. `ENERGYCHARTS_BASE_URL` and `UKCI_BASE_URL` are read from the environment.

### Changed
- The separate "Optimization potential" card is gone: its totals (estimated per month, verified, safe to automate vs. need
  approval) are a summary at the top of Top Recommendations, next to the list they sum up. The fourth KPI keeps the figure.

## [0.1.0-alpha.45] - 2026-10-10

### Added
- **Methodology page** (`/methodology`): the active version and its caveats, the formulas, the grid sources in the order
  used with what each is (measured, computed, monthly), and the accuracy and limits. The "read before using externally"
  notice and the Data quality card link to it.

### Changed
- The new Overview cards (data quality, optimization potential, workload efficiency, header, cost vs. carbon by service,
  regions, KPIs, recommendation cost reason) are translated: keys in both catalogs (ADR-0036).

## [0.1.0-alpha.44] - 2026-10-10

### Fixed
- `EMBER_API_KEY` and `EIA_API_KEY` in the environment were not read at all (only the console-stored key worked); they
  are now part of the integration environment map like the other keys.
- A main grid provider that does not return the past on its plan no longer fails the daily history loading (and its
  retries): the other sources still fill the past and the calculations are redone.

### Added
- `admin load-history`: starts the daily automatic loading of the past of regions with gaps right now.

## [0.1.0-alpha.43] - 2026-10-10

### Changed
- Grid intensity over time: a young history no longer sits as two dots at the far end of an empty month (the axis starts
  shortly before the first reading), and a note explains why few days have readings and what fills the past.
- Regions card: beside the map, a summary of where the footprint is, the cleanest and highest-carbon grid and the best
  move to evaluate. The map pop-up puts the map on the full width with the details and the list below it (no empty column).

## [0.1.0-alpha.42] - 2026-10-10

### Added
- **Cost vs. carbon by service**: a bubble chart (x share of cost, y share of CO2e, area cost); a click on a bubble
  scrolls to and highlights that service in Details by Service.
- **Regions: Map / List**. The list has cloud, exact region ID, grid intensity with its 24 h change, cost, CO2e and the
  possible improvement (a clearly cleaner region of the same cloud; price, latency and residency marked as not checked).
- **Overview header** with the scope of every figure: project, period and how fresh the data is.
- **Automation counters** in the optimization card: how many open recommendations are safe to automate (time shifting)
  and how many need approval (region moves, production changes), plus the jobs in progress and done.
- Chart cards link to their detail sections; the project and period filters are remembered between sections and visits.

### Changed
- The fourth KPI is **Optimization potential** (estimated kg CO2e per month, not verified) instead of SCI; per-unit
  figures live in Workload efficiency, which names the unit.
- "Total CO2e" is now **Operational CO2e** with a hint that it is not a life-cycle assessment; Carbon intensity says it is an
  energy-weighted average.

## [0.1.0-alpha.41] - 2026-10-10

### Changed
- **Overview loads faster**: the dot-matrix land of the region map is precomputed (`frontend/lib/land-dots.json`,
  generated by `scripts/gen-land-dots.ts`) instead of being calculated in the browser on first show; the map is a lazy
  chunk with a placeholder of its exact size; the world topology is no longer in the charts bundle.
- The map pop-up opens without recomputing or replaying the sweep, the canvas is sized to the screen, hover repaints only
  the dots of one country, and the heavy background blur is gone.

## [0.1.0-alpha.40] - 2026-10-10

### Changed
- **Region map rebuilt for speed and feel**: the land dots are drawn on one canvas (about 2500 animated SVG circles
  before), only transform and opacity are animated, and hover no longer redraws the map. In the pop-up you can zoom
  (wheel or buttons) and drag; choosing a region flies the camera to it; the country of the hovered region lights up;
  markers and labels keep their size while zooming; markers pop in one after another; the selected region gets a slowly
  turning ring. Honours reduced-motion.

## [0.1.0-alpha.39] - 2026-10-10

### Fixed
- The expanded region map opened inside its card (a transformed ancestor captured the fixed overlay) with a broken
  list. It is now a real pop-up over the page; a click on a marker opens it on that region, and every region the account
  uses is listed, also those without a grid reading yet.

## [0.1.0-alpha.38] - 2026-10-10

### Added
- **Optimization potential** on the Overview: what open and approved recommendations could save per month (estimated,
  carbon always, cost only where priced) next to what applied ones were MEASURED to save (impact verification). The two
  are never added together. `GET /dashboard/optimization`.
- **Workload efficiency**: cost and CO2e per unit of work over the project-days where units were reported, with the unit
  named; refused (with the reason) when no units were reported or the selected projects count different things.
  `GET /dashboard/efficiency`.
- **Map layers**: grid intensity, my footprint (marker size = cost in the period, colour = grid intensity) and relocation
  (a cleaner region of the same cloud, at least 15% cleaner, with the carbon it could save in the period). The detail
  panel shows cost, energy and operational CO2e of the region and lists what is NOT yet checked (price, latency, data
  residency, availability); the cost effect of a move is never guessed. `GET /dashboard/regions/footprint`.
  API contract 0.24.0.

### Changed
- The SCI card names the unit of work it is per (and says so when projects mix units).
- A recommendation without a price shows "cost: awaiting price data" with the reason; its carbon effect is labelled per
  month.

## [0.1.0-alpha.37] - 2026-10-10

### Added
- **Data quality** block on the Overview: share of grid intensity that is measured, where it came from (source and basis),
  freshness of the last sync, connections that need attention; links to the methodology.

### Changed
- Carbon Intensity by Region lists the cloud and the exact region ID of every row (AWS and Google Cloud IDs were mixed
  without a label).
- "Provisional estimates" notice links to the methodology page.

## [0.1.0-alpha.36] - 2026-10-10

### Added
- **EIA as a grid source for the United States** ([ADR-0035](docs/adr/0035-multi-source-grid-intensity.md)): hourly net
  generation by fuel of a balancing authority (PJM, MISO, CISO, PACW, PACE, ERCO, SC ...) from EIA-930, free with a key.
  The intensity is COMPUTED from the generation mix with generic life-cycle factors per fuel (an estimate; imports
  between authorities are not accounted for) and is used behind a source that publishes a measured value. Integration
  `eia` (sealed key, console test), source id `eia`, default order Electricity Maps, UK Carbon Intensity, EIA, Ember.
  Migration 0041.
- **Region map redrawn** as a dot-matrix world in pastel colours: every country is tinted from mint (clean) to rose
  (high carbon) by the grid intensity of the cloud regions in it, markers pulse softly, and the map opens full-screen
  (icon in the corner) with a region list and a detail panel (cloud, region ID, intensity, change, time of reading).
  Around 60 AWS and Google Cloud regions are placed on the map.

### Changed
- The source selectors in the operator console and the settings list offer EIA.

## [0.1.0-alpha.35] - 2026-10-10

### Added
- **Recalculate carbon** from the console (Data sources → Which source is used): redoes every project's calculations over
  the last N days in the background. `POST /operator/carbon/recalculate`. API contract 0.23.0.
- **Origin of the grid intensity in reports**: the carbon report (CSV, JSON, PDF) has `intensity_g_per_kwh`,
  `intensity_source` and `intensity_basis` per row, with a note explaining each basis.
- **RLX-CCF-1** ([ADR-0040](docs/adr/0040-ccf-coefficients.md)): usage-based coefficients taken from Cloud Carbon Footprint's
  published AWS constants (0.00212 kWh per vCPU-hour, 0.000392 kWh per GB-hour, 0.0004745 kWh per GB-month, PUE 1.135),
  with the source file and the derivation in the set's notes.

### Changed
- The active methodology version is **RLX-CCF-1** (about +1% for usage-based compute). RLX-PROVISIONAL-1 remains available;
  recalculate to see results under the new version. Cost-based factors and embodied emissions are still not sourced.

## [0.1.0-alpha.35] - 2026-10-10

### Added
- **Automatic loading of a region's past** (daily): a region in use whose last N days (default 90) are less than 90% covered
  by grid data gets its past loaded from every source that can supply it (UK Carbon Intensity, the main provider's past data
  when its plan has it, Ember's monthly figures) and every project is then recalculated over the window, so stored carbon
  uses the data that now exists. Settings `carbon.auto_load_history` and `carbon.auto_load_days`; requests stay under the
  daily cap. Migration 0040.
- **Best time for a flexible job**: from stored forecasts (Electricity Maps, UK Carbon Intensity) the start time within 48 h
  whose duration has the lowest mean forecast intensity, and the saving against starting now; on Carbon & Energy and for CI
  pipelines. `GET /carbon/schedule`, `GET /ci/schedule`. It is a forecast of average intensity, labelled as advice. API
  contract 0.22.0.

### Changed
- UK Carbon Intensity forecasts are stored (as forecasts, never as readings) for planning.
- Ember's terms are described with the licence discrepancy visible (CC-BY 4.0 on its site, CC BY-SA cited elsewhere).

## [0.1.0-alpha.35] - 2026-10-10

### Added
- **UK Carbon Intensity** as a free hourly source for Great Britain (no key; National Grid ESO): half-hourly measured values
  for the cloud regions whose zone is GB, refreshed hourly, loaded for the past with the same backfill, second in the
  default source order. It is "ready" with no setup. Migration 0039 (default order).

## [0.1.0-alpha.35] - 2026-10-10

### Added
- **Ember as a free second grid source** ([ADR-0039](docs/adr/0039-ember-and-climate-trace.md)): monthly national carbon
  intensity (free key, CC-BY 4.0), set up in Data sources, refreshed daily for the regions in use, with a key test. It fills
  days that have no hourly reading and is labelled "monthly national average" in the chart, the origin card and reports.
  Default source order: Electricity Maps, then Ember.
- **Climate TRACE** is listed as evaluated: its public API gives annual emission totals in tonnes without generation, so it
  cannot provide gCO2e/kWh; the registry says so.
- **Source per region** table in the console: for each region in use, what each source has, which one is used now, and a
  selector to pin a source (replaces the free-text `region=source` list).
- **Cost of serving an organization**: per organization, counts of usage rows, calculations, sync runs, deliveries, reports
  and Kubernetes samples over 30 days with an estimated cost from unit costs in the platform settings (new group "Cost of
  serving an organization"). `GET /operator/usage`.
- `GET /operator/carbon/regions`. API contract 0.21.0. Migrations 0037 and 0038.

### Changed
- Calculation rows may carry the intensity basis `month_mean`.

## [0.1.0-alpha.35] - 2026-10-10

### Added
- **Cost control for paid APIs**: every request to the grid data provider is counted per day (and failures), and a daily
  cap (platform setting `carbon.max_requests_per_day`, default 2000, 0 = unlimited) stops further requests until the
  next UTC day; stored readings are used meanwhile. The source page shows today against the cap and the last 7 and 30
  days. `GET /operator/api-usage`. API contract 0.20.0. Migration 0036.

## [0.1.0-alpha.35] - 2026-10-10

### Changed
- Payment provider settings moved to where they are looked for: **Plans & billing → Payment providers** (each opens its own
  settings page). They are no longer listed among the platform services under Data sources.

## [0.1.0-alpha.35] - 2026-10-10

### Added
- **Subscriptions and payments** ([ADR-0038](docs/adr/0038-payments-and-subscriptions.md)): a subscription per organization
  (plan, status active / trial / past due / ended, trial end, paid-until date); the scheduler returns an organization to Free
  when its period has ended. Operators set status and dates when assigning a plan.
- **Payment providers**, set up with keys in the console (Data sources → Platform services): **Stripe**, **Paddle**
  (merchant of record), **LiqPay** and **WayForPay** (Ukraine, UAH), **payment link** (Payoneer, Wise, PayPal.me …) and
  **invoice**. Apple Pay and Google Pay are offered on the providers' payment pages. Notifications are verified by signature,
  applied once, and visible to the operator on Plans & billing. `POST /plan/checkout`, `POST /billing/webhooks/{provider}`.
- Customer **Settings → Plan**: plan and status, usage against limits with bars, modules, change plan at a provider that is
  set up. `GET /plan`.
- Operator **Methodology & SCI** page: every coefficient set with provenance, coefficients, caveats and checksum.
  `GET /operator/methodology`. API contract 0.19.0. Migration 0035.

## [0.1.0-alpha.35] - 2026-10-10

### Added
- **Custom roles per organization** ([ADR-0037](docs/adr/0037-custom-roles.md)): define a role as a named set of
  permissions (Settings → Roles), give it to a member in Members (one select holds built-in and custom roles), withdraw it by
  choosing another role, delete it (members return to their built-in role). Guard rails: administration, data export and
  machine permissions can never be in a custom role; you can only grant permissions you hold; owners are not given custom
  roles; every change is audited. `GET/POST /roles`, `PUT/DELETE /roles/{id}`, `PATCH /members/{id}` with
  `custom_role_id`. API contract 0.18.0. Migration 0034.
- The public `GET /locales` honours the platform language settings (the languages on offer and the default).

## [0.1.0-alpha.35] - 2026-10-10

### Added
- **Multi-language platform** ([ADR-0036](docs/adr/0036-multi-language-platform.md)): English and Russian. Language
  switcher in both headers (cookie, falling back to the browser's language), `<html lang>`, catalogs with parity tests,
  plural forms, numbers, money and dates written by language. API problems carry a stable `code` and a translated `title`
  (`Accept-Language`). Settings `i18n.default_locale` and `i18n.enabled_locales`. The menus, headers and Carbon & Energy
  headings are translated; other page bodies are translated as their keys are added (English fallback).
- **Load past grid data** (Data sources → Which source is used): fetches past hourly readings of the regions in use from the
  provider (a provider plan option), in 9-day requests, with the number of requests shown before anything is asked and hard
  limits (400 days, 2000 requests). Days calculated before readings existed can then use measured intensity after a
  recalculation. `POST /operator/carbon/backfill`, job `carbon:backfill_intensity`. API contract 0.17.0. Migration 0033.

## [0.1.0-alpha.35] - 2026-10-10

### Added
- **Grid intensity from several sources** ([ADR-0035](docs/adr/0035-multi-source-grid-intensity.md)): a platform source policy
  (priority, a source per region, readings needed per day, oldest reading used) edited in the console next to the
  sources ("Which source is used"). For each region and day the first trusted source with enough readings wins; sources
  are never averaged together and marginal readings are never mixed with average ones.
- Every calculation row records the **source and basis** of its intensity (day mean, partial day, last reading before,
  latest overall, live). Rows calculated earlier stay unknown. Migration 0032.
- Client **Carbon & Energy**: grid intensity history per region (chosen source, daily range, other sources beside it, gaps
  left as gaps) and a card saying where the figures' intensities came from with the share that is a same-day mean.
  `GET /carbon/intensity`; the carbon summary carries `intensity_origins`. API contract 0.16.0.

### Changed
- Dashboard region intensities and recommendation inputs read the most trusted source (not "the latest of any provider").

## [0.1.0-alpha.35] - 2026-10-10

### Added
- Operator roles can be assigned and withdrawn from **Access & security** ([ADR-0034](docs/adr/0034-operator-roles-from-the-console.md)):
  add a person who has signed in by e-mail, change a role, withdraw access. The database refuses to change your own role
  and to leave the platform without an admin; every change is in the audit log. `PUT /operator/operators`,
  `POST /operator/operators/withdraw`. API contract 0.15.0. Migration 0031.

## [0.1.0-alpha.35] - 2026-10-10

### Fixed
- **Mixed currencies.** Cost totals, the dashboard cost trend, cost by service, cost anomalies and the cost basis of
  recommendations summed amounts of different currencies together. They now use the dominant currency of the window
  (largest total absolute cost); other currencies are left out, never added. A test pins it.
- **Grid intensity of a day.** A day of usage was calculated with the last reading stored before the end of the day.
  It now uses the mean of that day's readings, and falls back to the latest reading only when the day has none.
  Re-run a calculation to apply it to stored history.

### Changed
- Operator console: **Integrations is merged into Data sources.** Every source and every platform service has its own page
  with its terms, state and settings together (`/operator/data-sources/<id>`); the old page redirects. System no longer
  repeats the integration list, an organization's suspension switch is not repeated in its settings list, and the
  source page links to Connections instead of repeating its list.
- Left menu scrollbar (client workspace and console): thin, inset from the edge, a little wider under the pointer.

### Added
- Suspended organizations are skipped by the scheduler fan-out (no synchronization or calculation jobs). Migration 0030.
- A hand-computed reference test of the carbon arithmetic (energy, PUE, intensity, SCI shares).

## [0.1.0-alpha.35] - 2026-10-10

### Added
- Plan catalog and assignment ([ADR-0033](docs/adr/0033-plan-catalog-and-assignment.md)): Free, Team, Business and
  Enterprise as presets of limits and module switches; **Plans & billing** page with the comparison and the number of
  organizations per plan; assign a plan on an organization's page with a required reason (locked values in the
  settings history, plan and reason in the organization's audit log). `GET /operator/plans`,
  `PUT /operator/tenants/{id}/plan`. API contract 0.13.0. Migration 0029.

## [0.1.0-alpha.35] - 2026-10-10

### Added
- Suspend and restore an organization from its console page. Suspension is the locked operator setting
  `organization.suspended` with a message that people see: they keep read access and exports, every change is refused
  with 403 until an operator restores access. The change is in the settings history and the organization's audit log.
  Background synchronization of a suspended organization is not stopped yet (see the gap list).

## [0.1.0-alpha.35] - 2026-10-10

### Added
- Integrations for sources that are planned: **WattTime** (user name and password) and **UK Carbon Intensity API**
  (address, no key). They are marked "Prepared · not used yet": the values are stored sealed and no platform code reads
  them until the connector is released; the data source registry links to them.

## [0.1.0-alpha.35] - 2026-10-10

### Changed
- Operator console uses the same frame as the client workspace: the dark left panel with the same logo, links and
  version, the white header and a white content area. The pastel theme of alpha.18 is removed.

## [0.1.0-alpha.35] - 2026-10-10

### Added
- Operator console **Jobs & scheduler**: queues (waiting, running, scheduled, retrying, gave up, done and failed today,
  oldest wait), tasks that failed with their last error, and the periodic schedules the scheduler reported. Read-only;
  payloads are never returned. `GET /operator/jobs`.
- Operator console **Access & security**: the operators with role, date added and recent activity (read-only on purpose).
  `GET /operator/operators`. API contract 0.12.0. Migration 0028.

## [0.1.0-alpha.35] - 2026-10-10

### Added
- Operator console rebuilt ([ADR-0032](docs/adr/0032-operator-console-structure.md)): a calm pastel theme, a grouped menu
  (Overview, Customers, Data, Operations, Governance), page search from the header, a mobile drawer and animated tiles
  that respect reduced motion.
- **Overview**: counts, a "needs attention" list built from the real errors, failing connections, failed deliveries,
  missing integrations and failing data sources, and the latest operator activity.
- **Data sources**: registry of everything the platform reads (access type, metrics, coverage, terms, state). State comes
  from the real configuration and connection counts from real customer connections; planned sources are listed and carry
  no numbers. `GET /operator/data-sources`.
- **Audit log**: the operators' own access log, newest first, with a "changes only" view. `GET /operator/audit`.
- Menu entries for sections that are not built yet (plans and billing, aggregation and policies, methodology and SCI,
  jobs and scheduler, access and security) open a page that says what exists today and what is next.
- API contract 0.11.0. Migration 0027 (two read-only SECURITY DEFINER functions).

- `scripts/stand-deploy.sh`: builds and restarts the stand in one step, and applies the newest migration to the stand
  database (`docs/setup/stand.md`).

### Fixed
- Stand: pages answered with a chunk-loading error when the frontend was rebuilt under a running server; the deploy script
  restarts the web unit after every build.

### Changed
- Organizations moved to `/operator/organizations` and can be sorted and narrowed to those with problems; `/operator` is
  now the Overview.

## [0.1.0-alpha.35] - 2026-10-10

### Added
- Kubernetes right-sizing recommendations from measured usage ([ADR-0031](docs/adr/0031-kubernetes-rightsizing.md)): smaller
  CPU and memory requests for workloads whose peak hourly usage is far below what they request, with the measured peak next
  to the request; no saving is claimed. API contract 0.10.0. Migration 0026.
- Google Cloud can be connected from the Connectors page (project, billing export table, setup steps).

### Changed
- Cloud Accounts and Connectors are one page: connected accounts and the connect flow on top, the catalog below.
  `/cloud` redirects to `/connectors`; the account page stays at `/cloud/<id>`.

## [0.1.0-alpha.16] - 2026-10-10

### Added
- Kubernetes usage next to energy: optional CPU time, memory and the requests per workload and hour; the Kubernetes page
  shows CPU and memory utilization (used / requested, over the hours that report both; unknown stays a dash and low use
  is flagged). The agent reads cAdvisor and kube-state-metrics when present. API contract 0.9.0. Migration 0025.
- Systemd user units for a demonstration stand (`deploy/systemd`, `docs/setup/stand.md`) and `SCHEDULER_SKIP` to keep a
  stand without real cloud accounts from scheduling cloud synchronization.

## [0.1.0-alpha.15] - 2026-10-10

### Added
- Console **Integrations** now also hold the settings that are read when a service starts: the platform AWS account,
  the platform Google Cloud project and service account, object storage (S3) and error tracking / tracing. They are
  marked "Applied at restart" and win over the environment ([ADR-0029](docs/adr/0029-integration-credentials-in-the-console.md)).
- Monthly carbon limit per project (project policy, `alert_carbon_monthly_kg`) with its own carbon alert; project policy
  screen (allowed regions, CI gate thresholds, carbon limit) on the Projects page. API contract 0.8.0. Migration 0024.
- Packaged Kubernetes agent image `reliabilix/agent` (built by the release workflow, runs as an unprivileged user).

### Changed
- Sentry and tracing start after the database is reachable so they can use console values.

## [0.1.0-alpha.14] - 2026-10-10

### Added
- Kubernetes energy ([ADR-0030](docs/adr/0030-kubernetes-energy-agent.md)): a customer-side agent pushes measured workload
  energy (Kepler via Prometheus) with an API key of the new role `agent`; stored hourly per workload, carbon from the
  stored grid intensity and the active PUE (unknown stays unknown); Kubernetes page with clusters and workloads. It is
  attribution inside the cloud energy and is never added to totals. `POST /kubernetes/energy`, `GET /kubernetes/clusters`,
  `GET /kubernetes/workloads`; reference agent `scripts/kepler-push.sh` and `docs/setup/kubernetes.md`.
  API contract 0.7.0. Migration 0023.

## [0.1.0-alpha.13] - 2026-10-10

### Changed
- Sign-in page rebuilt: one calm form (user name and password in the test environment; Google, GitHub and e-mail with
  the identity provider) on real photographs: a defocused city of towers dissolving into a green forest, with a slow
  zoom and drifting mist. Photographs are CC0 and credited in `docs/credits.md`.

## [0.1.0-alpha.12] - 2026-10-10

### Changed
- Sign-in page redesigned as one full-screen industrial scene: dark control-room look with an engineering grid, cost and
  carbon telemetry traces, region nodes joined by flowing lines, server racks with status lights and instrument panels;
  the form sits in the middle on a light panel. Calm motion, switched off for people who prefer reduced motion.

## [0.1.0-alpha.11] - 2026-10-10

### Added
- Development sign-in on the product's own page (`DEV_LOGIN_PAGE=true` with `DEV_AUTH` and `DEV_BASIC_AUTH`): the
  browser's password prompt is replaced by the sign-in page; a signed http-only cookie carries the session; rate limited;
  absent outside the development mode.

### Fixed
- Sign out answered 404 in the development environment; `/auth/logout` now ends the session and returns to the sign-in page.

## [0.1.0-alpha.10] - 2026-10-10

### Added
- Operator console **Integrations** page ([ADR-0029](docs/adr/0029-integration-credentials-in-the-console.md)): the
  Electricity Maps key and zone overrides and the SMTP server, credentials and sender are set in the console, win over
  the environment, take effect without a restart, secrets are stored sealed and never shown, and each has a connection
  test. Fields fixed by the deployment are listed read-only with their state. API contract 0.6.0. Migration 0022.

### Changed
- The e-mail channel is now configured at run time: it is hidden (production) or only logs (development) until the
  server and the sender are set.

## [0.1.0-alpha.9] - 2026-10-10

### Added
- Sign-in page in the GreenOps style: landscape illustration, Google and GitHub buttons, e-mail entry that hands over
  to the identity provider (password and the two-step code are typed only there), password recovery form
  (`/api/auth/forgot-password`, same answer for any address, rate limited), responsive, with a clear development-mode
  notice. `/login?preview` shows it in the development environment.

## [0.1.0-alpha.8] - 2026-10-10

### Added
- Alert settings per organization: the warning level (`alerts.warn_pct`, default 80 %) and a monthly carbon limit
  (`alerts.carbon_monthly_kg`, default 0 = off) that opens a `carbon` alert for the calendar month so far
  ([ADR-0028](docs/adr/0028-alerts.md)). Migration 0021.

## [0.1.0-alpha.7] - 2026-10-10

### Added
- Alerts ([ADR-0028](docs/adr/0028-alerts.md)): stored alerts with a lifecycle (new, acknowledged, muted, resolved),
  opened and resolved by an hourly evaluation of budgets (80 % warning, 100 % critical) and connection health
  (errors, cost data that does not match the cloud); `alert.opened` notification; acknowledge/mute on the Alerts page.
  API contract 0.5.0. Migration 0020.

### Changed
- UI: softer card shadows and a fainter fading divider for a calmer, more uniform look.

## [0.1.0-alpha.6] - 2026-10-10

### Added
- Cloud account page (`/cloud/<id>`): connection state, the result of the last cost reconciliation with the cloud
  (matches / does not match / not checked, mismatched days), full sync history with per-run data quality, "Test
  connection" and "Sync now".

### Fixed
- `docker-compose.yml`: MinIO images come from `quay.io` (Docker Hub no longer serves `minio/minio`; the server image
  ships `mc`), so the local stack starts again.

## [0.1.0-alpha.5] - 2026-10-10

### Added
- Data lifecycle ([ADR-0027](docs/adr/0027-data-lifecycle.md)): owner-only export of all organization data (NDJSON,
  no secrets), erasure of an organization after a cancellable grace period (database in one transaction, files first,
  personal-data-free tombstone), and platform retention of usage and carbon history by whole months
  (`retention.data_months`, `lifecycle.deletion_grace_days`). Settings "Data" tab for owners.
- Object storage `DeletePrefix` (filesystem, memory, S3).

### Changed
- UI: white page background, slightly stronger card borders, a faint vertical rule that fades at both ends between the
  Overview content and its right column. The sidebar footer shows the full pre-release version (for example `alpha.5`),
  taken from the build.
- API contract 0.4.0 (export and deletion endpoints, additive).

## [0.1.0-alpha.4] - 2026-10-10

### Added
- Methodology registry: several immutable coefficient sets, one active; a test pins each file's checksum so a published
  set cannot be edited. `GET /carbon/methodology/versions`.
- Shadow recalculation under another version (`methodology_version` on `POST /carbon/recalculate`) and a read-only
  comparison of two versions over a window (`GET /carbon/compare`) ([ADR-0026](docs/adr/0026-methodology-registry-and-comparison.md)).

## [0.1.0-alpha.3] - 2026-10-10

### Added
- GCP cost data quality: every sync is reconciled with a second, ungrouped BigQuery query on the billing export
  ([ADR-0024](docs/adr/0024-cost-data-quality-reconciliation.md)). The AWS FOCUS export path stays unchecked on purpose.

## [0.1.0-alpha.2] - 2026-10-10

### Added
- Release workflow: a `v*` tag builds and pushes the five images with exact-version and build tags, never `latest`.
- Verified impact: an applied recommendation is measured (14 days before vs 14 days after, 3 days of billing lag
  skipped) and gets status `verified` with a separate result row; forecast and measurement are never mixed. New
  `verification` field on recommendations, `verified` status filter, `recommendation.verified` notification
  ([ADR-0025](docs/adr/0025-verified-impact.md)); cost data quality is documented in [ADR-0024](docs/adr/0024-cost-data-quality-reconciliation.md).
- Client menu completed to the 12 agreed items: Carbon & Energy (totals, trend, methodology and caveats), Connectors
  (catalog with honest availability), Alerts (cost anomalies, failing connections, undelivered notifications), CI/CD
  (placeholder with the CI gate). Pages without a backend say so.
- Product specification split into parts: `docs/product/` (client workspace, operator console, connectors, data and
  calculations, stack, gap analysis).
- Cost data quality: after each sync the stored daily totals are reconciled with the cloud's own totals (AWS Cost
  Explorer first); mismatching days rewind the sync cursor so the next sync re-reads them; the result is shown per run
  and on the connection, and raises a `connection.data_quality` notification.
- Release and version policy, version manifest, `/version`, `build_info` metric, OCI image labels.

### Security
- AWS: pagination cap is now an error instead of a silent truncation; FOCUS export reading is bounded (total decompressed
  bytes, line length, compressed size) and pinned to the connection's AWS account (`ExpectedBucketOwner`).
- FOCUS records reject NaN/Inf amounts and malformed currency codes before they reach the database.

### Changed
- API contract 0.3.0: methodology versions and comparison endpoints, optional `methodology_version` on recalculate (additive).
- API contract 0.2.0: optional `verification` on recommendations and the `verified` status (additive).
- A failed carbon-calculation enqueue after a successful sync no longer fails (and re-bills) the whole sync.

## [0.1.0-alpha.1] - 2026-10-09

First alpha. It was never tagged on its own: its content shipped as part of alpha.2, the first tagged build. Everything below exists but is provisional; carbon figures use the unverified `RLX-PROVISIONAL-1` set.

### Added
- Multi-tenant core (organizations, members, invitations, API keys, audit log, RLS with least-privilege database roles).
- Connectors: AWS (Cost Explorer, FOCUS data export, rightsizing), GCP (Cloud Billing export in BigQuery with ownership
  proof), Electricity Maps (grid intensity).
- FOCUS v1.4 ingestion, usage- and cost-based carbon engine with SCI, versioned methodology, recommendations
  (region shift, rightsizing) with human approval, automation plans, reports (CSV/JSON/PDF), CI gate.
- Notifications: producer port, e-mail / webhook / Slack channels, database outbox with retries ([ADR-0019](docs/adr/0019-notifications-port-and-channels.md)).
- Operator console and error log ([ADR-0020](docs/adr/0020-operator-console.md), [ADR-0021](docs/adr/0021-error-log.md)).
- Settings engine: platform / organization / operator scopes, locks, history, limits, module switches, maintenance and
  sign-up gates ([ADR-0023](docs/adr/0023-settings-engine.md)).
- Client and operator admin screens; dashboard redesigned after the design mock-up.
