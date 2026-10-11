-- +goose Up
-- The organization's own name can be changed by its administrators. The api role has no UPDATE on the organizations
-- table (and must not), so the one change is a function: it acts only on the tenant of the current transaction
-- (app.tenant_id, set by the same code path that applies row-level security) and validates the name itself.
-- +goose StatementBegin
CREATE FUNCTION tenants.rename_organization(p_name text) RETURNS text
LANGUAGE plpgsql SECURITY DEFINER SET search_path = pg_catalog, pg_temp AS $$
DECLARE oid uuid := NULLIF(current_setting('app.tenant_id', true), '')::uuid; clean text := btrim(p_name);
BEGIN
  IF oid IS NULL THEN RAISE EXCEPTION 'no tenant in context' USING ERRCODE = 'P0001'; END IF;
  IF clean IS NULL OR char_length(clean) < 2 OR char_length(clean) > 100 THEN
    RAISE EXCEPTION 'organization name must be 2-100 characters' USING ERRCODE = 'P0001';
  END IF;
  UPDATE tenants.organizations SET name = clean WHERE id = oid;
  RETURN clean;
END $$;
-- +goose StatementEnd
REVOKE ALL ON FUNCTION tenants.rename_organization(text) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION tenants.rename_organization(text) TO greenops_api;

-- +goose Down
DROP FUNCTION tenants.rename_organization(text);
