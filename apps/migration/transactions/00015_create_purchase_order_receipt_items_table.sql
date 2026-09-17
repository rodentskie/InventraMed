-- +goose Up
CREATE TABLE purchase_order_receipt_items (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    purchase_order_receipt_id uuid NOT NULL REFERENCES purchase_order_receipts (id) ON DELETE CASCADE,
    purchase_order_item_id uuid NOT NULL REFERENCES purchase_order_items (id),
    quantity_received int NOT NULL DEFAULT 0 CHECK (quantity_received >= 0),
    quantity_damaged int NOT NULL DEFAULT 0 CHECK (quantity_damaged >= 0),
    quantity_returned int NOT NULL DEFAULT 0 CHECK (quantity_returned >= 0),
    notes text,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX idx_purchase_order_receipt_items_receipt_id ON purchase_order_receipt_items (purchase_order_receipt_id);
CREATE INDEX idx_purchase_order_receipt_items_item_id ON purchase_order_receipt_items (purchase_order_item_id);

-- +goose Down
DROP TABLE purchase_order_receipt_items;
