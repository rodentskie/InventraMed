-- +goose Up
CREATE TABLE suppliers (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    name text NOT NULL,
    contact_name text,
    email text,
    phone text,
    address text,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    deleted_at timestamptz
);

CREATE INDEX idx_suppliers_deleted_at ON suppliers (deleted_at);

-- +goose Down
DROP TABLE suppliers;
