-- +goose Up
-- Which kinds of recommendation a project lets the platform apply without a person's approval. Empty = none: everything
-- waits for approval. Only changes that are safe to repeat (time shifting a flexible job) can ever be listed; region
-- moves and changes to production infrastructure always require approval (enforced in the domain, not only here).
ALTER TABLE projects.policies ADD COLUMN auto_apply_types text[] NOT NULL DEFAULT '{}';

-- +goose Down
ALTER TABLE projects.policies DROP COLUMN auto_apply_types;
