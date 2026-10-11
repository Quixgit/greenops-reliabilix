-- +goose Up
-- A recommendation is identified by what it is about (kind, project, provider, service, current region), no longer by the region
-- it proposes. The proposed region follows the grid, so the same finding used to come back as a "new" one whenever the cleanest
-- region changed, and the old one stayed open beside it. This collapses those duplicates (keeping a decided one, else the newest)
-- and moves the remaining fingerprints to the new definition.
--
-- The fingerprint is sha256 of "type|project|provider|service|region", as the application computes it.
WITH ranked AS (
  SELECT id, status,
         row_number() OVER (PARTITION BY tenant_id, project_id, type, coalesce(provider, ''), coalesce(service_name, ''), coalesce(current_region, '')
                            ORDER BY (status <> 'open') DESC, created_at DESC, id DESC) AS rn
  FROM recommendations.recommendations)
DELETE FROM recommendations.recommendations r USING ranked WHERE r.id = ranked.id AND ranked.rn > 1 AND ranked.status = 'open';

WITH ranked AS (
  SELECT id,
         row_number() OVER (PARTITION BY tenant_id, project_id, type, coalesce(provider, ''), coalesce(service_name, ''), coalesce(current_region, '')
                            ORDER BY (status <> 'open') DESC, created_at DESC, id DESC) AS rn
  FROM recommendations.recommendations)
UPDATE recommendations.recommendations r
SET fingerprint = encode(sha256(convert_to(r.type || '|' || r.project_id::text || '|' || coalesce(r.provider, '') || '|' || coalesce(r.service_name, '') || '|' || coalesce(r.current_region, ''), 'UTF8')), 'hex')
FROM ranked WHERE r.id = ranked.id AND ranked.rn = 1;

-- +goose Down
-- Fingerprints are an internal key; there is nothing to restore.
SELECT 1;
