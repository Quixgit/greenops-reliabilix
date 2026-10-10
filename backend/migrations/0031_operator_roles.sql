-- +goose Up
-- Operator roles can be assigned and withdrawn from the console (ADR-0034). Identity stays a database fact: these are
-- the only runtime paths that write platform.operators, each a narrow SECURITY DEFINER function that
--   * requires an operator admin,
--   * only promotes a person who already signed in to the platform (a row in tenants.users, found by e-mail),
--   * refuses to change the caller's own role and refuses to leave the platform without an admin,
--   * writes a line in the operators' access log (append-only).
-- +goose StatementBegin
CREATE FUNCTION platform.operator_assign(p_actor text, p_email text, p_role text) RETURNS text
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path = pg_catalog, pg_temp AS $$
DECLARE sub text; cur text; admins int;
BEGIN
  IF platform.operator_role(p_actor) IS DISTINCT FROM 'admin' THEN RAISE EXCEPTION 'operator admin required' USING ERRCODE = 'P0001'; END IF;
  IF p_role NOT IN ('viewer','admin') THEN RAISE EXCEPTION 'role must be viewer or admin' USING ERRCODE = 'P0001'; END IF;
  SELECT o.auth_sub INTO sub FROM platform.operators o WHERE lower(o.email) = lower(p_email) LIMIT 1;
  IF sub IS NULL THEN
    SELECT u.auth_sub INTO sub FROM tenants.users u WHERE lower(u.email) = lower(p_email) LIMIT 1;
  END IF;
  IF sub IS NULL THEN RAISE EXCEPTION 'no user with this e-mail has signed in to the platform yet' USING ERRCODE = 'P0001'; END IF;
  IF sub = p_actor THEN RAISE EXCEPTION 'you cannot change your own role' USING ERRCODE = 'P0001'; END IF;
  SELECT role INTO cur FROM platform.operators WHERE auth_sub = sub;
  IF cur = 'admin' AND p_role <> 'admin' THEN
    SELECT count(*) INTO admins FROM platform.operators WHERE role = 'admin';
    IF admins <= 1 THEN RAISE EXCEPTION 'the platform needs at least one operator admin' USING ERRCODE = 'P0001'; END IF;
  END IF;
  INSERT INTO platform.operators (auth_sub, email, role) VALUES (sub, left(p_email, 320), p_role)
    ON CONFLICT (auth_sub) DO UPDATE SET role = EXCLUDED.role;
  INSERT INTO platform.operator_access_log (operator_sub, action)
    VALUES (p_actor, left('operator role set: ' || p_email || ' -> ' || p_role || ' (was ' || coalesce(cur, 'none') || ')', 300));
  RETURN sub;
END $$;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE FUNCTION platform.operator_withdraw(p_actor text, p_sub text) RETURNS boolean
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path = pg_catalog, pg_temp AS $$
DECLARE cur text; mail text; admins int;
BEGIN
  IF platform.operator_role(p_actor) IS DISTINCT FROM 'admin' THEN RAISE EXCEPTION 'operator admin required' USING ERRCODE = 'P0001'; END IF;
  IF p_sub = p_actor THEN RAISE EXCEPTION 'you cannot withdraw your own access' USING ERRCODE = 'P0001'; END IF;
  SELECT role, email INTO cur, mail FROM platform.operators WHERE auth_sub = p_sub;
  IF cur IS NULL THEN RETURN false; END IF;
  IF cur = 'admin' THEN
    SELECT count(*) INTO admins FROM platform.operators WHERE role = 'admin';
    IF admins <= 1 THEN RAISE EXCEPTION 'the platform needs at least one operator admin' USING ERRCODE = 'P0001'; END IF;
  END IF;
  DELETE FROM platform.operators WHERE auth_sub = p_sub;
  INSERT INTO platform.operator_access_log (operator_sub, action)
    VALUES (p_actor, left('operator withdrawn: ' || coalesce(nullif(mail, ''), p_sub) || ' (was ' || cur || ')', 300));
  RETURN true;
END $$;
-- +goose StatementEnd

GRANT INSERT, UPDATE, DELETE ON platform.operators TO greenops_fanout;
ALTER FUNCTION platform.operator_assign(text, text, text) OWNER TO greenops_fanout;
ALTER FUNCTION platform.operator_withdraw(text, text)     OWNER TO greenops_fanout;
REVOKE ALL ON FUNCTION platform.operator_assign(text, text, text), platform.operator_withdraw(text, text) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION platform.operator_assign(text, text, text), platform.operator_withdraw(text, text) TO greenops_api;

-- +goose Down
DROP FUNCTION platform.operator_withdraw(text, text), platform.operator_assign(text, text, text);
REVOKE INSERT, UPDATE, DELETE ON platform.operators FROM greenops_fanout;
