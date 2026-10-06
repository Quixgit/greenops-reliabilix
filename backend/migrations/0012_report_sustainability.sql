-- +goose Up
ALTER TABLE reports.reports DROP CONSTRAINT reports_kind_check;
ALTER TABLE reports.reports ADD CONSTRAINT reports_kind_check CHECK (kind IN ('carbon','sci','finops','sustainability'));

-- +goose Down
DELETE FROM reports.reports WHERE kind = 'sustainability';
ALTER TABLE reports.reports DROP CONSTRAINT reports_kind_check;
ALTER TABLE reports.reports ADD CONSTRAINT reports_kind_check CHECK (kind IN ('carbon','sci','finops'));
