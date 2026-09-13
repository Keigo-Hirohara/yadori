ALTER TABLE holds
    ADD CONSTRAINT fk_holds_inventory
    FOREIGN KEY (room_type_id, date) REFERENCES inventories (room_type_id, date);