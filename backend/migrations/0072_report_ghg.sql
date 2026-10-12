-- +goose Up
-- A GHG Protocol Scope 3 / ESRS E1 inventory report kind (ADR-0055).
ALTER TABLE reports.reports DROP CONSTRAINT reports_kind_check;
ALTER TABLE reports.reports ADD CONSTRAINT reports_kind_check CHECK (kind IN ('carbon','sci','finops','sustainability','ghg'));
ALTER TABLE reports.schedules DROP CONSTRAINT schedules_kind_check;
ALTER TABLE reports.schedules ADD CONSTRAINT schedules_kind_check CHECK (kind IN ('carbon','sci','finops','sustainability','ghg'));

-- +goose Down
DELETE FROM reports.schedules WHERE kind = 'ghg';
DELETE FROM reports.reports WHERE kind = 'ghg';
ALTER TABLE reports.schedules DROP CONSTRAINT schedules_kind_check;
ALTER TABLE reports.schedules ADD CONSTRAINT schedules_kind_check CHECK (kind IN ('carbon','sci','finops','sustainability'));
ALTER TABLE reports.reports DROP CONSTRAINT reports_kind_check;
ALTER TABLE reports.reports ADD CONSTRAINT reports_kind_check CHECK (kind IN ('carbon','sci','finops','sustainability'));
