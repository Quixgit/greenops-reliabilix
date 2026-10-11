-- +goose Up
-- The organization overview counted a connection as failing when its status was not 'ok', but the statuses are
-- 'healthy', 'error' and 'pending': every healthy connection was reported as failing, and the organization list (which
-- counts 'error') disagreed with the organization's own page. Failing means 'error', in both places.

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION platform.operator_org_overview(p_tenant uuid) RETURNS jsonb
LANGUAGE sql STABLE SECURITY DEFINER SET search_path = pg_catalog, pg_temp AS $$
  SELECT jsonb_build_object(
    'id', o.id, 'name', o.name, 'plan', o.plan, 'created_at', o.created_at,
    'members', (SELECT count(*) FROM tenants.memberships WHERE organization_id = o.id),
    'invitations_open', (SELECT count(*) FROM tenants.invitations WHERE organization_id = o.id AND accepted_at IS NULL AND expires_at > now()),
    'api_keys', (SELECT count(*) FROM tenants.api_keys WHERE organization_id = o.id AND revoked_at IS NULL),
    'projects', (SELECT count(*) FROM projects.projects WHERE tenant_id = o.id),
    'connections', (SELECT count(*) FROM cloudaccounts.connections WHERE tenant_id = o.id),
    'connections_failing', (SELECT count(*) FROM cloudaccounts.connections WHERE tenant_id = o.id AND sync_status = 'error'),
    'reports', (SELECT count(*) FROM reports.reports WHERE tenant_id = o.id),
    'destinations', (SELECT count(*) FROM notifications.destinations WHERE tenant_id = o.id),
    'gate_checks', (SELECT count(*) FROM carbon.gate_evaluations WHERE tenant_id = o.id),
    'last_activity', (SELECT max(created_at) FROM audit.audit_logs WHERE tenant_id = o.id),
    'erasure_requested', (SELECT jsonb_build_object('at', requested_at, 'execute_after', execute_after, 'by', requested_by)
                           FROM tenants.deletion_requests WHERE tenant_id = o.id))
  FROM tenants.organizations o WHERE o.id = p_tenant
$$;
-- +goose StatementEnd

-- +goose Down
-- The previous definition was wrong; there is nothing to go back to.
SELECT 1;
