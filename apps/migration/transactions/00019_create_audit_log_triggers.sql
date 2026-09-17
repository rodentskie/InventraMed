-- +goose Up
-- +goose StatementBegin
-- Reads the acting user from the "app.actor_id" session setting, which callers
-- set per-transaction (e.g. `SET LOCAL app.actor_id = '<uuid>'`). Falls back to
-- NULL for system-initiated changes when the setting isn't present.
CREATE FUNCTION log_audit_event() RETURNS trigger AS $$
DECLARE
    actor uuid;
BEGIN
    actor := NULLIF(current_setting('app.actor_id', true), '')::uuid;

    IF TG_OP = 'INSERT' THEN
        INSERT INTO audit_logs (entity_type, entity_id, action, changes, actor_id)
        VALUES (TG_TABLE_NAME, NEW.id, 'created', jsonb_build_object('after', to_jsonb(NEW)), actor);
        RETURN NEW;
    ELSIF TG_OP = 'UPDATE' THEN
        INSERT INTO audit_logs (entity_type, entity_id, action, changes, actor_id)
        VALUES (TG_TABLE_NAME, NEW.id, 'updated', jsonb_build_object('before', to_jsonb(OLD), 'after', to_jsonb(NEW)), actor);
        RETURN NEW;
    ELSIF TG_OP = 'DELETE' THEN
        INSERT INTO audit_logs (entity_type, entity_id, action, changes, actor_id)
        VALUES (TG_TABLE_NAME, OLD.id, 'deleted', jsonb_build_object('before', to_jsonb(OLD)), actor);
        RETURN OLD;
    END IF;
    RETURN NULL;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

CREATE TRIGGER trg_purchase_orders_audit_log
    AFTER INSERT OR UPDATE OR DELETE ON purchase_orders
    FOR EACH ROW EXECUTE FUNCTION log_audit_event();

CREATE TRIGGER trg_purchase_order_items_audit_log
    AFTER INSERT OR UPDATE OR DELETE ON purchase_order_items
    FOR EACH ROW EXECUTE FUNCTION log_audit_event();

CREATE TRIGGER trg_purchase_order_receipts_audit_log
    AFTER INSERT OR UPDATE OR DELETE ON purchase_order_receipts
    FOR EACH ROW EXECUTE FUNCTION log_audit_event();

CREATE TRIGGER trg_purchase_order_receipt_items_audit_log
    AFTER INSERT OR UPDATE OR DELETE ON purchase_order_receipt_items
    FOR EACH ROW EXECUTE FUNCTION log_audit_event();

-- +goose Down
DROP TRIGGER IF EXISTS trg_purchase_order_receipt_items_audit_log ON purchase_order_receipt_items;
DROP TRIGGER IF EXISTS trg_purchase_order_receipts_audit_log ON purchase_order_receipts;
DROP TRIGGER IF EXISTS trg_purchase_order_items_audit_log ON purchase_order_items;
DROP TRIGGER IF EXISTS trg_purchase_orders_audit_log ON purchase_orders;
DROP FUNCTION IF EXISTS log_audit_event();
