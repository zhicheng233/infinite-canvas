-- +goose Up
DELETE FROM credit_pricing WHERE deleted_at IS NOT NULL;

-- +goose Down
-- Historical soft-deleted compatibility projections are intentionally not restored.
