-- +goose Up
CREATE TABLE purchase_order_items (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    purchase_order_id uuid NOT NULL REFERENCES purchase_orders (id) ON DELETE CASCADE,
    medicine_id uuid NOT NULL REFERENCES medicines (id),
    quantity_ordered int NOT NULL CHECK (quantity_ordered > 0),
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX idx_purchase_order_items_purchase_order_id ON purchase_order_items (purchase_order_id);

-- +goose Down
DROP TABLE purchase_order_items;
