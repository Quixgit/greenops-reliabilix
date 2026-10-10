-- +goose Up
-- Settings engine (ADR-0023). The catalog of settings (keys, types, bounds, defaults) lives in code
-- (internal/platform/settings); these tables only hold the values that differ from the defaults, plus the history.
CREATE SCHEMA settings;

-- Platform-wide values. Readable by the api and worker (it is configuration, never a secret), written only through
-- operator functions.
CREATE TABLE settings.platform_values (
    key        text PRIMARY KEY CHECK (char_length(key) BETWEEN 3 AND 100),
    value      jsonb NOT NULL,
    updated_by text NOT NULL,
    updated_at timestamptz NOT NULL DEFAULT now()
);

-- Per-organization values. source 'tenant' = chosen by the organization; 'operator' = set by an operator, which also
-- LOCKS the key: the application roles can neither change nor delete such a row (restrictive policies below).
CREATE TABLE settings.tenant_values (
    tenant_id  uuid NOT NULL REFERENCES tenants.organizations(id),
    key        text NOT NULL CHECK (char_length(key) BETWEEN 3 AND 100),
    value      jsonb NOT NULL,
    source     text NOT NULL CHECK (source IN ('tenant','operator')),
    updated_by text NOT NULL,
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id, key)
);
SELECT platform.enable_tenant_rls('settings.tenant_values', 'tenant_id');
CREATE POLICY tenant_source_insert ON settings.tenant_values AS RESTRICTIVE FOR INSERT WITH CHECK (source = 'tenant');
CREATE POLICY tenant_source_update ON settings.tenant_values AS RESTRICTIVE FOR UPDATE USING (source = 'tenant') WITH CHECK (source = 'tenant');
CREATE POLICY tenant_source_delete ON settings.tenant_values AS RESTRICTIVE FOR DELETE USING (source = 'tenant');

-- Append-only record of every change. Rows of an organization are visible to it (RLS); platform-level rows
-- (tenant_id NULL) are visible to nobody but operators, through a function.
CREATE TABLE settings.history (
    id        bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    at        timestamptz NOT NULL DEFAULT now(),
    tenant_id uuid REFERENCES tenants.organizations(id),
    key       text NOT NULL,
    old_value jsonb,
    new_value jsonb,
    actor     text NOT NULL,
    via       text NOT NULL CHECK (via IN ('tenant','operator'))
);
CREATE INDEX ON settings.history (tenant_id, at DESC);
SELECT platform.enable_tenant_rls('settings.history', 'tenant_id');

-- ---- operator functions (owner greenops_fanout) ------------------------------------------------------------
-- +goose StatementBegin
CREATE FUNCTION settings.operator_set_platform(p_actor text, p_key text, p_value jsonb, p_reset boolean) RETURNS void
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path = pg_catalog, pg_temp AS $$
DECLARE old jsonb;
BEGIN
  IF platform.operator_role(p_actor) IS DISTINCT FROM 'admin' THEN RAISE EXCEPTION 'operator admin required' USING ERRCODE = 'P0001'; END IF;
  SELECT value INTO old FROM settings.platform_values WHERE key = p_key;
  IF p_reset THEN
    DELETE FROM settings.platform_values WHERE key = p_key;
    IF old IS NOT NULL THEN
      INSERT INTO settings.history (tenant_id, key, old_value, new_value, actor, via) VALUES (NULL, p_key, old, NULL, p_actor, 'operator');
    END IF;
  ELSE
    INSERT INTO settings.platform_values (key, value, updated_by) VALUES (p_key, p_value, p_actor)
      ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value, updated_by = EXCLUDED.updated_by, updated_at = now();
    IF old IS DISTINCT FROM p_value THEN
      INSERT INTO settings.history (tenant_id, key, old_value, new_value, actor, via) VALUES (NULL, p_key, old, p_value, p_actor, 'operator');
    END IF;
  END IF;
END $$;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE FUNCTION settings.operator_tenant_values(p_tenant uuid) RETURNS TABLE (key text, value jsonb, source text, updated_by text, updated_at timestamptz)
LANGUAGE sql STABLE SECURITY DEFINER SET search_path = pg_catalog, pg_temp AS $$
  SELECT v.key, v.value, v.source, v.updated_by, v.updated_at FROM settings.tenant_values v WHERE v.tenant_id = p_tenant ORDER BY v.key
$$;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE FUNCTION settings.operator_set_tenant(p_actor text, p_tenant uuid, p_key text, p_value jsonb, p_reset boolean) RETURNS boolean
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path = pg_catalog, pg_temp AS $$
DECLARE old jsonb;
BEGIN
  IF platform.operator_role(p_actor) IS DISTINCT FROM 'admin' THEN RAISE EXCEPTION 'operator admin required' USING ERRCODE = 'P0001'; END IF;
  IF NOT EXISTS (SELECT 1 FROM tenants.organizations WHERE id = p_tenant) THEN RETURN false; END IF;
  SELECT value INTO old FROM settings.tenant_values WHERE tenant_id = p_tenant AND key = p_key;
  IF p_reset THEN
    DELETE FROM settings.tenant_values WHERE tenant_id = p_tenant AND key = p_key AND source = 'operator';
    IF NOT FOUND THEN RETURN true; END IF;
    INSERT INTO settings.history (tenant_id, key, old_value, new_value, actor, via) VALUES (p_tenant, p_key, old, NULL, p_actor, 'operator');
    INSERT INTO audit.audit_logs (tenant_id, actor_user_id, action, target) VALUES (p_tenant, 'operator:' || p_actor, 'operator.setting_unlocked', 'setting:' || p_key);
  ELSE
    INSERT INTO settings.tenant_values (tenant_id, key, value, source, updated_by) VALUES (p_tenant, p_key, p_value, 'operator', p_actor)
      ON CONFLICT (tenant_id, key) DO UPDATE SET value = EXCLUDED.value, source = 'operator', updated_by = EXCLUDED.updated_by, updated_at = now();
    INSERT INTO settings.history (tenant_id, key, old_value, new_value, actor, via) VALUES (p_tenant, p_key, old, p_value, p_actor, 'operator');
    INSERT INTO audit.audit_logs (tenant_id, actor_user_id, action, target) VALUES (p_tenant, 'operator:' || p_actor, 'operator.setting_changed', 'setting:' || p_key);
  END IF;
  RETURN true;
END $$;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE FUNCTION settings.operator_history(p_tenant uuid, p_limit int)
RETURNS TABLE (at timestamptz, tenant_id uuid, key text, old_value jsonb, new_value jsonb, actor text, via text)
LANGUAGE sql STABLE SECURITY DEFINER SET search_path = pg_catalog, pg_temp AS $$
  SELECT h.at, h.tenant_id, h.key, h.old_value, h.new_value, h.actor, h.via FROM settings.history h
  WHERE (p_tenant IS NULL AND h.tenant_id IS NULL) OR h.tenant_id = p_tenant
  ORDER BY h.id DESC LIMIT least(greatest(p_limit, 1), 500)
$$;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE FUNCTION platform.purge_operator_access_log(p_retain interval) RETURNS bigint
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path = pg_catalog, pg_temp AS $$
DECLARE n bigint;
BEGIN
  IF p_retain < interval '90 days' THEN RAISE EXCEPTION 'the operator access log must be kept at least 90 days' USING ERRCODE = 'P0001'; END IF;
  DELETE FROM platform.operator_access_log WHERE at < now() - p_retain;
  GET DIAGNOSTICS n = ROW_COUNT;
  RETURN n;
END $$;
-- +goose StatementEnd

-- ---- grants -----------------------------------------------------------------------------------------------
GRANT USAGE ON SCHEMA settings TO greenops_api, greenops_worker, greenops_fanout;
GRANT SELECT ON settings.platform_values TO greenops_api, greenops_worker;
GRANT SELECT, INSERT, UPDATE, DELETE ON settings.tenant_values TO greenops_api;
GRANT SELECT ON settings.tenant_values TO greenops_worker;
GRANT SELECT, INSERT ON settings.history TO greenops_api;
GRANT SELECT ON settings.history TO greenops_worker;

GRANT SELECT, INSERT, UPDATE, DELETE ON settings.platform_values, settings.tenant_values TO greenops_fanout;
GRANT SELECT, INSERT ON settings.history TO greenops_fanout;
GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA settings TO greenops_fanout;
GRANT DELETE ON platform.operator_access_log TO greenops_fanout;

ALTER FUNCTION settings.operator_set_platform(text, text, jsonb, boolean)        OWNER TO greenops_fanout;
ALTER FUNCTION settings.operator_tenant_values(uuid)                             OWNER TO greenops_fanout;
ALTER FUNCTION settings.operator_set_tenant(text, uuid, text, jsonb, boolean)    OWNER TO greenops_fanout;
ALTER FUNCTION settings.operator_history(uuid, int)                              OWNER TO greenops_fanout;
ALTER FUNCTION platform.purge_operator_access_log(interval)                      OWNER TO greenops_fanout;
REVOKE ALL ON FUNCTION settings.operator_set_platform(text, text, jsonb, boolean), settings.operator_tenant_values(uuid),
  settings.operator_set_tenant(text, uuid, text, jsonb, boolean), settings.operator_history(uuid, int), platform.purge_operator_access_log(interval) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION settings.operator_set_platform(text, text, jsonb, boolean), settings.operator_tenant_values(uuid),
  settings.operator_set_tenant(text, uuid, text, jsonb, boolean), settings.operator_history(uuid, int) TO greenops_api;
GRANT EXECUTE ON FUNCTION platform.purge_operator_access_log(interval) TO greenops_worker;

-- +goose Down
DROP FUNCTION platform.purge_operator_access_log(interval), settings.operator_history(uuid, int),
  settings.operator_set_tenant(text, uuid, text, jsonb, boolean), settings.operator_tenant_values(uuid), settings.operator_set_platform(text, text, jsonb, boolean);
DROP TABLE settings.history;
DROP TABLE settings.tenant_values;
DROP TABLE settings.platform_values;
DROP SCHEMA settings;
