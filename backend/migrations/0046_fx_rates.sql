-- +goose Up
-- Daily euro reference rates (ECB), global reference data (no tenant): units of the currency per 1 EUR.
CREATE TABLE finops.fx_rates (
    day        date NOT NULL,
    currency   char(3) NOT NULL,
    per_eur    numeric(18,8) NOT NULL CHECK (per_eur > 0),
    source     text NOT NULL DEFAULT 'ecb',
    PRIMARY KEY (day, currency)
);
CREATE INDEX fx_rates_currency_day ON finops.fx_rates (currency, day DESC);
GRANT SELECT ON finops.fx_rates TO greenops_api, greenops_worker;
GRANT INSERT, UPDATE ON finops.fx_rates TO greenops_worker;

-- Converts an amount between two currencies at the latest reference rate on or before the day (a rate is carried over
-- weekends and holidays, at most 10 days). NULL when a rate is missing: the caller leaves such an amount out and says so,
-- it is never guessed. Same currency: unchanged.
-- +goose StatementBegin
CREATE FUNCTION finops.convert_cost(p_amount numeric, p_from text, p_to text, p_day date) RETURNS numeric
LANGUAGE sql STABLE PARALLEL SAFE SET search_path = pg_catalog, pg_temp AS $$
  SELECT CASE
    WHEN upper(trim(p_from)) = upper(trim(p_to)) THEN p_amount
    ELSE p_amount * rt.r / rf.r
  END
  FROM LATERAL (SELECT CASE WHEN upper(trim(p_from)) = 'EUR' THEN 1::numeric ELSE
         (SELECT per_eur FROM finops.fx_rates WHERE currency = upper(trim(p_from)) AND day <= p_day AND day > p_day - 10 ORDER BY day DESC LIMIT 1) END AS r) rf,
       LATERAL (SELECT CASE WHEN upper(trim(p_to)) = 'EUR' THEN 1::numeric ELSE
         (SELECT per_eur FROM finops.fx_rates WHERE currency = upper(trim(p_to)) AND day <= p_day AND day > p_day - 10 ORDER BY day DESC LIMIT 1) END AS r) rt
$$;
-- +goose StatementEnd
GRANT EXECUTE ON FUNCTION finops.convert_cost(numeric, text, text, date) TO greenops_api, greenops_worker;

-- +goose Down
DROP FUNCTION finops.convert_cost(numeric, text, text, date);
DROP TABLE finops.fx_rates;
