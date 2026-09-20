-- +goose Up
CREATE UNIQUE INDEX uq_suppliers_name
    ON suppliers (lower(name))
    WHERE deleted_at IS NULL;

-- +goose Down
DROP INDEX uq_suppliers_name;
