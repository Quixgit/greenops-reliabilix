-- +goose Up
-- Carbon threshold alerts (ADR-0028): a monthly carbon limit per organization raises an alert of kind 'carbon'.
ALTER TABLE alerts.alerts DROP CONSTRAINT alerts_kind_check;
ALTER TABLE alerts.alerts ADD CONSTRAINT alerts_kind_check CHECK (kind IN ('budget','carbon','connection_error','data_quality'));

-- +goose Down
DELETE FROM alerts.alerts WHERE kind = 'carbon';
ALTER TABLE alerts.alerts DROP CONSTRAINT alerts_kind_check;
ALTER TABLE alerts.alerts ADD CONSTRAINT alerts_kind_check CHECK (kind IN ('budget','connection_error','data_quality'));
