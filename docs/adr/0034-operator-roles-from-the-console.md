# ADR-0034: Operator roles are assigned and withdrawn from the console

Status: accepted · 0.1.0-alpha.34 · amends [ADR-0020](0020-operator-console.md)

## Context
ADR-0020 made "who is an operator" a fact only the migration owner could change (a command-line step), so that a
compromised API process cannot make anyone an operator. In practice that makes roles rigid: giving someone read access for
a day needs a deployment tool. The product owner asked for roles that can be assigned and withdrawn when needed.

## Decision
Two narrow `SECURITY DEFINER` functions are the only runtime path that writes `platform.operators`:
- `platform.operator_assign(actor, email, role)`: makes a person an operator or changes their role.
- `platform.operator_withdraw(actor, subject)`: withdraws access.

Guard rails, enforced in the database and not only in the API:
1. the caller must be an operator `admin`;
2. the person is found **by e-mail** among people who have already signed in (`tenants.users`) or among existing operators;
   an unknown address creates nothing;
3. a caller cannot change or withdraw themselves;
4. the platform always keeps at least one admin;
5. every change is a line in the append-only operators' access log, next to the request line the API writes.

The role set stays `viewer` and `admin`. The console is the place: Access & security (`PUT /operator/operators`,
`POST /operator/operators/withdraw`).

## Consequences
- A compromised API process can now call these functions, but only with the subject of an existing operator admin, so it
  needs an admin session. That is the same power an admin already has in the console; it is the accepted trade-off.
- The command-line tools (`make operator-add`) keep working and remain the way to create the first admin.
- Organization roles (owner, admin, engineer, billing, viewer) are unchanged and managed in each organization's Settings.
  A custom per-organization permission matrix is a separate, later decision.
