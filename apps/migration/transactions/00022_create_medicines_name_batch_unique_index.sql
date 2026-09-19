-- +goose Up
CREATE UNIQUE INDEX uq_medicines_name_batch
    ON medicines (lower(name), coalesce(lower(batch_number), ''))
    WHERE deleted_at IS NULL;

-- +goose Down
DROP INDEX uq_medicines_name_batch;
