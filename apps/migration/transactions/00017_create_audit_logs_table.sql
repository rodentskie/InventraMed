-- +goose Up
CREATE TABLE audit_logs (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    entity_type text NOT NULL,
    entity_id uuid NOT NULL,
    action text NOT NULL,
    changes jsonb,
    actor_id uuid REFERENCES users (id),
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX idx_audit_logs_entity ON audit_logs (entity_type, entity_id);

-- +goose Down
DROP TABLE audit_logs;
