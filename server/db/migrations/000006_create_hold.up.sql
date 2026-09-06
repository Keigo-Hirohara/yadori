CREATE TYPE hold_status AS enum (
    'temporary_hold',
    'processing_payment',
    'confirmed'
);

CREATE TABLE holds (
    id UUID PRIMARY KEY,
    room_type_id UUID NOT NULL REFERENCES room_types(id),
    date DATE NOT NULL,
    booking_id UUID NOT NULL REFERENCES bookings(id),
    status hold_status NOT NULL DEFAULT 'temporary_hold',
    expired_at TIMESTAMPTZ,
    slot_no      INT  NOT NULL CHECK (slot_no >= 1),
    CONSTRAINT uq_slot UNIQUE (date, room_type_id, slot_no),
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);