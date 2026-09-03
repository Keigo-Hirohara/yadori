CREATE TABLE bookers (
    id             UUID PRIMARY KEY,
    first_name     TEXT NOT NULL,
    last_name      TEXT NOT NULL,
    postal_code    TEXT NOT NULL,
    phone_number   TEXT NOT NULL,
    prefecture     TEXT NOT NULL,
    city           TEXT NOT NULL,
    street_address TEXT NOT NULL,
    building       TEXT NOT NULL DEFAULT '',
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);