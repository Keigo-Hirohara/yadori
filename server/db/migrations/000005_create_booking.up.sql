CREATE TYPE booking_status AS enum (
    'temporary_hold',
    'processing_payment',
    'cancelled',
    'confirmed'
);

CREATE TABLE bookings (
    id UUID PRIMARY KEY,
    booker_id UUID NOT NULL REFERENCES bookers(id),
    room_type_id UUID NOT NULL REFERENCES room_types(id),
    total_fee INT NOT NULL CHECK (total_fee >= 0),
    checkin_date DATE NOT NULL,
    checkout_date DATE NOT NULL,
    status booking_status NOT NULL DEFAULT 'temporary_hold',
    CONSTRAINT valid_period CHECK (checkout_date > checkin_date),
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);