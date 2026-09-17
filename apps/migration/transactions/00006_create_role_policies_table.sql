-- +goose Up
CREATE TABLE role_policies (
    role_id uuid NOT NULL REFERENCES roles (id) ON DELETE CASCADE,
    policy_id uuid NOT NULL REFERENCES policies (id) ON DELETE CASCADE,
    PRIMARY KEY (role_id, policy_id)
);

-- +goose Down
DROP TABLE role_policies;
