-- +goose Up
CREATE TABLE settings (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    warning_threshold_days int NOT NULL DEFAULT 30
);

-- +goose Down
DROP TABLE settings;
