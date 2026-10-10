-- DEV ONLY. Creates an empty tenant so the UI can be opened without Auth0 (DEV_AUTH=true).
-- Matches DEV_TENANT_ID in docker-compose.yml. No usage data: Overview shows its honest empty states.
INSERT INTO tenants.organizations (id, name, plan)
VALUES ('00000000-0000-4000-8000-000000000001', 'Dev Organization', 'free')
ON CONFLICT DO NOTHING;

-- DEV ONLY: the dev login ("dev-user", see frontend/lib/auth/session.ts) is also a platform operator, so the operator
-- console at /operator can be opened locally. Production operators are added by hand: `make operator-add`.
INSERT INTO platform.operators (auth_sub, email, role)
VALUES ('dev-user', 'dev@reliabilix.local', 'admin')
ON CONFLICT DO NOTHING;
