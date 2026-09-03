CREATE TABLE room_types (
    id               UUID PRIMARY KEY,
    accommodation_id UUID NOT NULL REFERENCES accommodations(id),
    name             TEXT NOT NULL,
    capacity         INT  NOT NULL CHECK (capacity >= 1),
    has_private_bath BOOLEAN NOT NULL DEFAULT false,
    has_balcony      BOOLEAN NOT NULL DEFAULT false,
    deleted_at       TIMESTAMPTZ,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);
