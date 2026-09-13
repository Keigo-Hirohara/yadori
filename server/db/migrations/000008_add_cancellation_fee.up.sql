ALTER TABLE bookings
    ADD COLUMN cancellation_fee INT NOT NULL DEFAULT 0 CHECK (cancellation_fee >= 0);
