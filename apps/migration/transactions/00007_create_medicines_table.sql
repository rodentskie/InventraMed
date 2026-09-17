-- +goose Up
CREATE TABLE medicines (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    name text NOT NULL,
    barcode text NOT NULL UNIQUE,
    batch_number text,
    expiration_date date NOT NULL,
    quantity int NOT NULL,
    created_by uuid REFERENCES users (id),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    deleted_at timestamptz
);

CREATE INDEX idx_medicines_deleted_at ON medicines (deleted_at);

-- +goose Down
DROP TABLE medicines;
