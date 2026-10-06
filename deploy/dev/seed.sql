-- DEV ONLY. Creates an empty tenant so the UI can be opened without Auth0 (DEV_AUTH=true).
-- Matches DEV_TENANT_ID in docker-compose.yml. No usage data: Overview shows its honest empty states.
INSERT INTO tenants.organizations (id, name, plan)
VALUES ('00000000-0000-4000-8000-000000000001', 'Dev Organization', 'free')
ON CONFLICT DO NOTHING;
