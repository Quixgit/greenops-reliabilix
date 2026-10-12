-- +goose Up
-- Savings verification with a control group (ADR-0054). The raw before/after change stays as it was; next to it the measurement now
-- records the change corrected for the trend of the rest of the project (the control), how that trend moved, and over how many days.
ALTER TABLE recommendations.impact_verifications
    ADD COLUMN adjusted_carbon_kg_month numeric(18,3),
    ADD COLUMN adjusted_cost_month      numeric(18,2),
    ADD COLUMN control_growth_pct       numeric(8,2),
    ADD COLUMN control_days             int NOT NULL DEFAULT 0 CHECK (control_days >= 0);

-- +goose Down
ALTER TABLE recommendations.impact_verifications
    DROP COLUMN control_days, DROP COLUMN control_growth_pct, DROP COLUMN adjusted_cost_month, DROP COLUMN adjusted_carbon_kg_month;
