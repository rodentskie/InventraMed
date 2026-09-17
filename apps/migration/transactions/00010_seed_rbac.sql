-- +goose Up
-- +goose StatementBegin
INSERT INTO roles (name, description) VALUES
    ('admin', 'Full access to all resources'),
    ('standard', 'Standard user; no access to RBAC or settings management');
-- +goose StatementEnd

-- +goose StatementBegin
INSERT INTO policies (name, effect, action, resource) VALUES
    ('admin-full-access', 'allow', '*', '*'),
    ('standard-allow-all', 'allow', '*', '*'),
    ('standard-deny-rbac', 'deny', '*', 'rbac:*'),
    ('standard-deny-settings', 'deny', '*', 'settings:*');
-- +goose StatementEnd

-- +goose StatementBegin
INSERT INTO role_policies (role_id, policy_id)
SELECT r.id, p.id
FROM roles r
JOIN policies p ON (
    (r.name = 'admin' AND p.name = 'admin-full-access')
    OR (r.name = 'standard' AND p.name IN ('standard-allow-all', 'standard-deny-rbac', 'standard-deny-settings'))
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DELETE FROM policies
WHERE name IN ('admin-full-access', 'standard-allow-all', 'standard-deny-rbac', 'standard-deny-settings');
-- +goose StatementEnd

-- +goose StatementBegin
DELETE FROM roles WHERE name IN ('admin', 'standard');
-- +goose StatementEnd
