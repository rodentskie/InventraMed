-- +goose Up
CREATE TRIGGER trg_medicines_set_updated_at
    BEFORE UPDATE ON medicines
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- +goose Down
DROP TRIGGER IF EXISTS trg_medicines_set_updated_at ON medicines;
