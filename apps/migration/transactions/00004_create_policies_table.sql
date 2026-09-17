-- +goose Up
CREATE TABLE policies (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    name text NOT NULL UNIQUE,
    effect text NOT NULL CHECK (effect IN ('allow', 'deny')),
    action text NOT NULL,
    resource text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    deleted_at timestamptz
);

CREATE INDEX idx_policies_deleted_at ON policies (deleted_at);

-- +goose Down
DROP TABLE policies;
