# ADR-0057: Terraform provider and the `iac` API key

Status: accepted

## Context
Phase 5 asks to build the platform into a customer's process: a public API with examples and a Terraform provider. The API is documented by
`openapi.yaml`, but the keys a machine could hold were CI (evaluate only), viewer (read all), agent and SCIM. Nothing let code manage projects or
budgets without a person's account.

## Decision
**A key of its own: role `iac`** (migration 0074). Permissions: project read and write, billing read, budget write. Nothing else: no cloud connections, members,
keys, settings, notifications, automation or approval (tested, and checked live: 403 on those routes). It is made in Settings like any key, by whoever
manages members.

**The provider** (`terraform-provider-reliabilix/`, its own Go module, `terraform-plugin-framework`) has the resources `reliabilix_project`, `reliabilix_project_policy`,
`reliabilix_budget` and the data source `reliabilix_project`. It talks to the public API only. Because the API has no update for projects and budgets, a change replaces them.
The policy resource writes only what it manages and carries the automatic-application list through unchanged, read just before the write, so applying
Terraform can never switch automatic changes on or off. Deleting a policy resource clears regions and thresholds. A resource gone from the platform is dropped from
the state and recreated at the next apply; drift is shown by plan.

## Verified
Against the stand with the real `terraform` binary: apply of all four kinds, an empty second plan, an in-place policy change, drift made through the API detected and
corrected, a budget deleted outside recreated, import, destroy, a wrong key and a missing endpoint failing with a clear message. Unit tests of the client.

## Not done
Publishing to a registry and signed release builds; resources for notification destinations, teams, environments, allocation rules and cloud connections (a
connection needs a secret handshake with the cloud and is not a good fit for code); update endpoints that would avoid replacement; acceptance tests in CI.
