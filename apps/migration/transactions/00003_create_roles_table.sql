-- +goose Up
CREATE TABLE roles (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    name text NOT NULL UNIQUE,
    description text,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    deleted_at timestamptz
);

CREATE INDEX idx_roles_deleted_at ON roles (deleted_at);

-- +goose Down
DROP TABLE roles;
