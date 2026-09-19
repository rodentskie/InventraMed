-- +goose Up
-- +goose StatementBegin
INSERT INTO users (email, password_hash, name) VALUES
    ('admin@local.com', '$2a$10$jcUt/H5pvLq7FEeKkPpC4ekc2P6zeOnALMY46ZFtgb2SYPzXQZ2M2', 'Admin');
-- +goose StatementEnd

-- +goose StatementBegin
INSERT INTO user_roles (user_id, role_id)
SELECT u.id, r.id
FROM users u
JOIN roles r ON r.name = 'admin'
WHERE u.email = 'admin@local.com';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DELETE FROM users WHERE email = 'admin@local.com';
-- +goose StatementEnd
