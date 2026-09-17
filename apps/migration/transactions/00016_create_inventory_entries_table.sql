-- +goose Up
CREATE TABLE inventory_entries (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    medicine_id uuid NOT NULL REFERENCES medicines (id),
    direction text NOT NULL CHECK (direction IN ('addition', 'subtraction')),
    quantity int NOT NULL CHECK (quantity > 0),
    reason text NOT NULL,
    counted_by uuid NOT NULL REFERENCES users (id),
    notes text,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX idx_inventory_entries_medicine_id ON inventory_entries (medicine_id);

-- +goose Down
DROP TABLE inventory_entries;
