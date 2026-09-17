-- +goose Up
CREATE TABLE purchase_orders (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    supplier_id uuid NOT NULL REFERENCES suppliers (id),
    status text NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'ordered', 'partially_received', 'received', 'cancelled')),
    order_date date NOT NULL,
    expected_date date,
    created_by uuid NOT NULL REFERENCES users (id),
    notes text,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    deleted_at timestamptz
);

CREATE INDEX idx_purchase_orders_deleted_at ON purchase_orders (deleted_at);
CREATE INDEX idx_purchase_orders_supplier_id ON purchase_orders (supplier_id);

-- +goose Down
DROP TABLE purchase_orders;
