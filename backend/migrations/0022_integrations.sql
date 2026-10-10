-- +goose Up
-- Integration values (ADR-0029): the keys and addresses of external services (Electricity Maps, SMTP) that an
-- operator can set in the console instead of in the deployment environment. Secret values are stored sealed
-- (AES-256-GCM, see platform/secretbox): the database never holds one in clear text, and the API never returns one.
CREATE TABLE platform.integration_values (
    integration text NOT NULL CHECK (integration ~ '^[a-z0-9_]{1,40}$'),
    key         text NOT NULL CHECK (key ~ '^[a-z0-9_]{1,40}$'),
    value       text NOT NULL CHECK (char_length(value) BETWEEN 1 AND 4000),
    secret      boolean NOT NULL,
    updated_by  text NOT NULL,
    updated_at  timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (integration, key)
);

-- Operators change values only through this function (the API role has no write grant on the table), so every change
-- is one audited code path.
-- +goose StatementBegin
CREATE FUNCTION platform.operator_set_integration(p_actor text, p_integration text, p_key text, p_value text, p_secret boolean, p_reset boolean)
RETURNS void LANGUAGE plpgsql SECURITY DEFINER SET search_path = pg_catalog, pg_temp AS $$
BEGIN
  IF p_reset THEN
    DELETE FROM platform.integration_values WHERE integration = p_integration AND key = p_key;
  ELSE
    INSERT INTO platform.integration_values (integration, key, value, secret, updated_by)
    VALUES (p_integration, p_key, p_value, p_secret, p_actor)
    ON CONFLICT (integration, key) DO UPDATE
      SET value = EXCLUDED.value, secret = EXCLUDED.secret, updated_by = EXCLUDED.updated_by, updated_at = now();
  END IF;
END $$;
-- +goose StatementEnd

GRANT SELECT, INSERT, UPDATE, DELETE ON platform.integration_values TO greenops_fanout;
GRANT USAGE ON SCHEMA platform TO greenops_fanout;
ALTER FUNCTION platform.operator_set_integration(text, text, text, text, boolean, boolean) OWNER TO greenops_fanout;
REVOKE ALL ON FUNCTION platform.operator_set_integration(text, text, text, text, boolean, boolean) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION platform.operator_set_integration(text, text, text, text, boolean, boolean) TO greenops_api;
GRANT SELECT ON platform.integration_values TO greenops_api, greenops_worker;

-- +goose Down
DROP FUNCTION platform.operator_set_integration(text, text, text, text, boolean, boolean);
DROP TABLE platform.integration_values;
