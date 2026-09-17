-- +goose Up
CREATE TABLE purchase_order_receipts (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    purchase_order_id uuid NOT NULL REFERENCES purchase_orders (id) ON DELETE CASCADE,
    received_by uuid NOT NULL REFERENCES users (id),
    received_at timestamptz NOT NULL DEFAULT now(),
    notes text,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX idx_purchase_order_receipts_purchase_order_id ON purchase_order_receipts (purchase_order_id);

-- +goose Down
DROP TABLE purchase_order_receipts;
