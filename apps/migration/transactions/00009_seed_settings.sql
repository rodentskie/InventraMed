-- +goose Up
INSERT INTO settings (warning_threshold_days) VALUES (30);

-- +goose Down
DELETE FROM settings;
