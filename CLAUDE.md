# Project instructions (Reliabilix GreenOps)

## Versions and releases (keep them current)

The platform is pre-release. Current line and stage: see `VERSION` (alpha, `0.1.0-alpha.N`). The rules are in
`docs/release/versioning.md`; the numbers of every service, module, connector, contract and container image are in
`VERSIONS.md`; the history is in `CHANGELOG.md`.

Whenever you change something that deserves a changelog line, in the same change:

1. Add the line under **Unreleased** in `CHANGELOG.md` (Added / Changed / Fixed / Security / Breaking).
2. Update `VERSIONS.md`: a new or removed service, module, package or connector gets or loses its row; a migration sets
   `data/db-schema`; an API / settings / webhook / methodology change bumps its contract row. In alpha every row carries
   the platform alpha number and moves together; from beta on rows move independently (policy section 3).
3. When cutting a release: update `VERSION`, `frontend/package.json` `version`, `backend/api/openapi.yaml` `info.version`
   (if the API changed), move Unreleased under the new version with the date, and tell the user the new version, the
   stage, and the image tags per service.
4. Never publish or document a `latest` image tag; production uses exact version tags.
5. Run `go test ./internal/platform/version/...`: it fails when the manifest and the repository disagree.

When the stage would change (alpha -> beta -> rc -> stable), check the entry criteria in the policy first and record the
result in the changelog; do not change the stage on your own.

## Conventions

- Backend: Go modular monolith; domains never import each other (ports wired in `internal/app`); every tenant query runs
  in `database.WithTenantTx`; secrets never in the database in clear text (sealed only) and never in the frontend.
- External services: a key or an address of an external service gets an entry in `platform/integrations` (catalog,
  console, sealed secrets) in the same change as the code that uses it; read it through the store, never from `os.Getenv`
  at the call site (ADR-0029).
- Settings: add a `Def` to `platform/settings/catalog.go` only together with the code that honours it.
- No `Co-Authored-By` lines in commit messages (user's global rule).
