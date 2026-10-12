# ADR-0049: WattTime client for marginal intensity, prepared but not connected

Status: accepted (client only; the connection waits for an account)

## Context
Carbon-aware scheduling by the emissions of the generator that would actually change with a job (marginal intensity,
WattTime's `co2_moer` signal) is on the roadmap next to the average intensity every other source gives
([ADR-0035](0035-multi-source-grid-intensity.md): a reading has a measure, and only `average` feeds carbon figures). The
platform has no WattTime account yet, and the integration already accepts the user name and password in the operator console
(catalog entry `watttime`, sealed).

## Decision
- `internal/carbon/providers/watttime` is a tested client of API v3: `GET /login` with HTTP Basic for a token (renewed
  after 25 of the 30 minutes it lives, and once more when a call is rejected), `/v3/region-from-loc`, `/v3/historical`
  (requested in 30-day chunks), `/v3/forecast` (1-72 hours) and `/v3/my-access` for a "Test" button.
- Values are converted to **g CO2e/kWh** from the unit in `meta.units`. A response without a unit, or with one the client does
  not know, is refused: the unit is not assumed. Absurd values and points without a value are skipped.
- Errors never carry the credentials, the URL query or the body; "bad credentials" (401 on login) and "no access to this
  region or signal" (403, a plan limit; WattTime previews only one region without a subscription) are different errors.
- Inputs (region code, coordinates, horizon) are validated before any request; the host is the configured base address only and
  redirects are not followed.
- Nothing calls the client in the running system. The catalog entry stays "planned", and its note says the client exists.

## What is left when an account exists
1. Give the carbon module's `HourlyExtra` port a measure (it saves every extra reading as `average` today), so a WattTime
   reading is stored as `marginal` and the source policy never resolves a figure from it. Add `watttime` to the
   priority-list enum only if marginal sources are allowed there at all (they should not be).
2. Map a cloud region to a WattTime region with the coordinates in `platform/geo` and `Region`, cache the answer.
3. Add `GET /ci/schedule?measure=marginal`, fed to `domain.ChooseWindow` (it works on any forecast points; its comment says the
   figure is average, and must say the measure then). Show marginal and average under their own names in the console.
4. Verify the units against the live API with a real key before turning it on; the conversion table has one entry for
   pounds per MWh because that is what the signal is documented to use, and nothing else has been seen.

## Consequences
The risky parts (the login flow, unit handling, error mapping) are tested now, so connecting WattTime later is the
three small changes above plus a key, not a new integration.
