-- +goose Up
-- A monthly carbon limit per project (ADR-0028): the alerts evaluation opens a carbon alert for the project when its
-- emissions in the calendar month so far approach or pass it. NULL = no project limit.
ALTER TABLE projects.policies
    ADD COLUMN alert_carbon_monthly_kg numeric(18,3) CHECK (alert_carbon_monthly_kg IS NULL OR alert_carbon_monthly_kg >= 0);

-- +goose Down
ALTER TABLE projects.policies DROP COLUMN alert_carbon_monthly_kg;
