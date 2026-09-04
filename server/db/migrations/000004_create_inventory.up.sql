CREATE TABLE inventories (
    id UUID PRIMARY KEY,
    room_type_id UUID NOT NULL REFERENCES room_types(id),
    date DATE NOT NULL,
    fee INT NOT NULL CHECK (fee >= 0),
    quantity_available INT NOT NULL CHECK (quantity_available >= 0),
    is_closed BOOLEAN DEFAULT false NOT NULL,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT unique_inventory_group UNIQUE (date, room_type_id)
);