# Versioning and releases

Reliabilix is **pre-release**: the current line is `0.1.0-alpha.94`. This document is the policy; the actual numbers
live in [`VERSIONS.md`](../../VERSIONS.md) and the history in [`CHANGELOG.md`](../../CHANGELOG.md).

## 1. Release stages

Version = [SemVer 2.0](https://semver.org): `MAJOR.MINOR.PATCH[-PRERELEASE]`. The pre-release identifier is the stage.

| Stage | Version looks like | What it means | Who may use it | Compatibility promise |
|---|---|---|---|---|
| **alpha** | `0.1.0-alpha.N` | Features are still being added; known gaps are listed in the roadmap; figures are provisional | the team and invited testers, never customer data that matters | none: APIs, settings and tables may change between alphas |
| **beta** | `0.1.0-beta.N` | Feature-complete for the release; stabilising: bug fixes, security review, live-integration checks | design partners with real accounts | no breaking API changes without a changelog "Breaking" entry and one beta of notice; migrations stay forward-only |
| **rc** | `0.1.0-rc.N` | Candidate: only release-blocking fixes | pilot customers | frozen; a fix is a new rc |
| **stable** | `0.1.0` | Released | everyone | SemVer from here on |

`N` increases by one for every tagged build of that stage (`alpha.1`, `alpha.2` ...). Going from alpha to beta resets
`N` to 1 and keeps `MAJOR.MINOR.PATCH` (`0.1.0-alpha.94` -> `0.1.0-beta.1` -> `0.1.0-rc.1` -> `0.1.0`).

**Entry criteria for beta:** every connector verified against a live account (not only fakes); `data/methodology`
replaced by a citable dataset (or the UI says provisional); error log and operator console running in staging for two
weeks; restore from backup rehearsed; the open items in the security model's pre-production list closed or accepted
in writing. **For rc:** no open severity-1 or -2 findings, upgrade from the previous beta tested with real data.
**For stable:** a pen test report with no unresolved high findings.

The UI shows the platform version as `v0.1.0` with a stage badge (alpha / beta / rc); stable shows no badge.

## 2. What is versioned, and how independently

Everything in `VERSIONS.md` has its own version:

- the **platform release** (one number for the whole product, also the `VERSION` file and `frontend/package.json`),
- every **service** (api, worker, scheduler, migrate, web, admin CLI) and its **container image**,
- every backend **domain module** and **platform package**,
- every **connector and channel plug-in** (AWS, GCP, Electricity Maps, e-mail, webhook, Slack, and the ones still to come),
- the **contracts and data** others depend on: the API contract, the webhook payload and signature, FOCUS, the database
  schema, the carbon methodology, the settings catalog,
- the **third-party base images** (pinned, never floating).

**While in alpha** every service, module and connector carries the platform version: they move together, which keeps the
manifest honest and releases simple. **From beta onward they are independent**: a version changes only when that thing
changes, and the platform release is the tested *combination* (the manifest of that release is the bill of materials).

## 3. When a version bumps

| Thing | MAJOR | MINOR | PATCH |
|---|---|---|---|
| service / module / connector | breaking change to its API, port, config or stored data | new capability, backwards compatible | fix, performance, internal change |
| `contract/api` | a path, field or meaning removed or changed | an endpoint or optional field added | description or example fixes |
| `contract/webhook-*`, `contract/focus` | an integer that only counts incompatible revisions (receivers branch on it) | n/a | n/a |
| `data/db-schema` | number of the newest migration; changes with every migration | | |
| `data/methodology` | a new coefficient set is a **new version name** (RLX-PROVISIONAL-2 ...); an existing one is never edited | | |
| `data/settings-catalog` | a setting removed or its meaning changed | settings added | wording |
| platform release | see section 1 | | |

In alpha, a breaking change is allowed but must appear under **Breaking** in the changelog. A change to a setting key, a
migration that cannot be rolled forward, an API removal, or a changed webhook field is always a breaking change.

## 4. Container images

- Names: `<registry>/reliabilix/<service>`; services `api`, `worker`, `scheduler`, `migrate`, `web` and `agent` (the Kubernetes energy agent, which runs in the customer's cluster).
- Tags per build: the exact version (`reliabilix/api:0.1.0-alpha.94`) **and** an immutable build tag
  (`0.1.0-alpha.94-<git sha7>`). Moving tags are allowed only as conveniences for humans: `0.1` (latest patch of a minor),
  `alpha` / `beta` (latest build of a stage). **Production deploys pin the exact version tag (or the digest); `latest` is
  never used and is never published.**
- Every image carries OCI labels: `org.opencontainers.image.version`, `.revision`, `.title`, `.source`, `.created`.
- The binaries know who they are: the Dockerfile injects `platform/version.Platform`, `.Service` and `.Commit` with
  `-ldflags`; they are printed at start-up, exported as the Prometheus gauge `build_info{service,version,commit}` and
  served at `GET /version`.
- `docker-compose.yml` builds and tags the same names (`VERSION` and `COMMIT` build arguments, `REGISTRY` prefix), so a
  local stack and a release use identical image names.
- Third-party images are pinned to a major version at least (`postgres:18`, `redis:7-alpine`); a bump is a changelog entry.

## 5. Compatibility rules that make independent versions safe

1. **Database migrations are forward-only and additive** (new columns nullable or with defaults, new tables). A destructive
   change is split over two releases: stop using, then drop. The `migrate` image of release N runs before any service of
   release N starts; services of release N-1 must keep working against schema N (rolling upgrade).
2. **The API stays under `/api/v1`.** Within a major version fields are only added. The OpenAPI test keeps spec and routes equal.
3. **Webhook receivers** branch on `version` in the payload; a new field is not a new version, a removed or re-typed one is.
4. **Settings keys are permanent**; a renamed setting is a new key plus a migration of stored values.
5. **The methodology is immutable**; reports print the version they used.

## 6. How a release is made

1. Decide the version (sections 1 and 3). Update, in one commit: `VERSION`, `VERSIONS.md` (every row that changed),
   `frontend/package.json` `version`, `backend/api/openapi.yaml` `info.version` if the API changed, and `CHANGELOG.md`
   (move **Unreleased** under the new version with the date).
2. `go test ./...` runs the manifest consistency test: it fails when those files disagree, when a service, module or
   package has no row in `VERSIONS.md`, or when `data/db-schema` is not the newest migration.
3. Tag `v<version>` (for example `v0.1.0-alpha.94`). CI builds the six images with `VERSION` and `COMMIT`, tags them as in
   section 4 and pushes them. The release notes are the changelog section.
4. Deploy by pinning the exact tags from the manifest of that release. Run `migrate` first. Check `GET /version` on api and
   the `build_info` metric of every service.
5. Roll back by redeploying the previous manifest's tags; migrations are not rolled back (rule 1).

## 7. Keeping the manifest current (for people and for the assistant)

Every change that is worth a line in the changelog also touches `VERSIONS.md`:

- added or removed a service, module, package or connector: add or remove its row;
- changed behaviour of one: bump it as in section 3 (in alpha: the platform alpha number, so all rows move together);
- added a migration: set `data/db-schema` to its number;
- changed the API, settings catalog, webhook payload or methodology: bump the matching contract row;
- changed a base image: update the `base/*` row.

Releases are never skipped silently: when the stage changes (alpha -> beta), the entry criteria in section 1 are checked
and the result is written in the changelog. The repository's `CLAUDE.md` tells the assistant to follow this section.
