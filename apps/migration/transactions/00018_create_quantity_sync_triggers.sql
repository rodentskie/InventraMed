-- +goose Up
-- +goose StatementBegin
CREATE FUNCTION sync_medicine_quantity_from_receipt_item() RETURNS trigger AS $$
BEGIN
    UPDATE medicines
    SET quantity = quantity + NEW.quantity_received
    WHERE id = (
        SELECT medicine_id FROM purchase_order_items WHERE id = NEW.purchase_order_item_id
    );
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

CREATE TRIGGER trg_purchase_order_receipt_items_sync_quantity
    AFTER INSERT ON purchase_order_receipt_items
    FOR EACH ROW
    EXECUTE FUNCTION sync_medicine_quantity_from_receipt_item();

-- +goose StatementBegin
CREATE FUNCTION sync_medicine_quantity_from_inventory_entry() RETURNS trigger AS $$
BEGIN
    IF NEW.direction = 'addition' THEN
        UPDATE medicines SET quantity = quantity + NEW.quantity WHERE id = NEW.medicine_id;
    ELSE
        UPDATE medicines SET quantity = quantity - NEW.quantity WHERE id = NEW.medicine_id;
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

CREATE TRIGGER trg_inventory_entries_sync_quantity
    AFTER INSERT ON inventory_entries
    FOR EACH ROW
    EXECUTE FUNCTION sync_medicine_quantity_from_inventory_entry();

-- +goose Down
DROP TRIGGER IF EXISTS trg_inventory_entries_sync_quantity ON inventory_entries;
DROP FUNCTION IF EXISTS sync_medicine_quantity_from_inventory_entry();
DROP TRIGGER IF EXISTS trg_purchase_order_receipt_items_sync_quantity ON purchase_order_receipt_items;
DROP FUNCTION IF EXISTS sync_medicine_quantity_from_receipt_item();
