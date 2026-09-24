-- +goose Up
ALTER TABLE medicines
    ADD COLUMN location smallint
        CONSTRAINT chk_medicines_location CHECK (location BETWEEN 1 AND 12);

CREATE UNIQUE INDEX uq_medicines_location
    ON medicines (location)
    WHERE deleted_at IS NULL AND location IS NOT NULL;

-- +goose Down
DROP INDEX uq_medicines_location;
ALTER TABLE medicines DROP COLUMN location;
