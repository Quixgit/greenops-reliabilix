-- +goose Up
-- Operator console: assign a plan to an organization. The plan's limits and module switches are written through the
-- settings engine by the API; this function records the plan on the organization and the reason in the
-- organization's own audit log, like every other operator action (ADR-0020, ADR-0033).
-- +goose StatementBegin
CREATE FUNCTION platform.operator_set_plan(p_actor text, p_tenant uuid, p_plan text, p_reason text) RETURNS boolean
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path = pg_catalog, pg_temp AS $$
DECLARE n int; old text;
BEGIN
  IF platform.operator_role(p_actor) IS DISTINCT FROM 'admin' THEN RAISE EXCEPTION 'operator admin required' USING ERRCODE = 'P0001'; END IF;
  IF p_plan !~ '^[a-z][a-z0-9_-]{1,31}$' THEN RAISE EXCEPTION 'invalid plan' USING ERRCODE = 'P0001'; END IF;
  IF char_length(btrim(p_reason)) < 3 OR char_length(p_reason) > 300 THEN RAISE EXCEPTION 'a reason of 3-300 characters is required' USING ERRCODE = 'P0001'; END IF;
  SELECT plan INTO old FROM tenants.organizations WHERE id = p_tenant;
  UPDATE tenants.organizations SET plan = p_plan WHERE id = p_tenant;
  GET DIAGNOSTICS n = ROW_COUNT;
  IF n = 0 THEN RETURN false; END IF;
  INSERT INTO audit.audit_logs (tenant_id, actor_user_id, action, target, metadata)
    VALUES (p_tenant, 'operator:' || p_actor, 'operator.plan_changed', 'plan:' || p_plan,
            jsonb_build_object('from', coalesce(old, ''), 'to', p_plan, 'reason', left(btrim(p_reason), 300)));
  RETURN true;
END $$;
-- +goose StatementEnd
ALTER FUNCTION platform.operator_set_plan(text, uuid, text, text) OWNER TO greenops_fanout;
REVOKE ALL ON FUNCTION platform.operator_set_plan(text, uuid, text, text) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION platform.operator_set_plan(text, uuid, text, text) TO greenops_api;

-- +goose Down
DROP FUNCTION platform.operator_set_plan(text, uuid, text, text);
