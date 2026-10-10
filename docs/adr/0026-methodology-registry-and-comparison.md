# ADR-0026: Methodology registry, shadow recalculation and version comparison

Status: accepted (implemented)

## Context
ADR-0009 made the methodology a versioned, immutable coefficient set, but the code knew exactly one set and read it
as a global. A second set (a citable dataset replacing the provisional one) could not be introduced without rewriting
history, and nobody could see what a new set would have changed before switching to it.

## Decision
- **Registry.** Every `*.json` file in `platform/methodology` is one set, looked up by version; `ActiveVersion` names
  the one new results, dashboards and reports use. A test pins the SHA-256 of each file: editing a published set fails
  the build, a new set must be a new file with a new version name and a new pinned line.
- **Calculation under a chosen version.** `ComputeWith(factors, ...)` and `POST /carbon/recalculate` with
  `methodology_version`. Rows of that version only are replaced (the unique key already contains the version), so a
  *shadow run* of a candidate set lives next to the active one. Shadow runs do not queue recommendations.
- **Comparison, read only.** `GET /carbon/compare?a=&b=` reads the stored rows of two versions over one window. It never
  calculates; a version without rows says `has_data: false`, and a delta is returned only when both cover the same days.
- **Stored result is its own snapshot.** Each row keeps the energy, the grid intensity that was used and the version,
  so a result can be explained without the coefficient file of the day; usage rows are kept, so any version can be
  recomputed from them.

## Consequences
- Switching the active version is a code change plus a recalculation window; old results stay readable by version.
- Dashboards, reports and recommendations still read the active version only; reading an older version there is
  deliberately not offered until a customer needs it.
- Intensity history is kept in `carbon.grid_intensity`, so a recomputed window uses the reading of that time, not today's.
