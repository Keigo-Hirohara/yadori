-- name: UpsertBooking :exec
INSERT INTO bookings (
    id, booker_id, room_type_id, total_fee, checkin_date, checkout_date, status, cancellation_fee
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8
)
ON CONFLICT (id) DO UPDATE SET
    total_fee        = EXCLUDED.total_fee,
    checkin_date     = EXCLUDED.checkin_date,
    checkout_date    = EXCLUDED.checkout_date,
    status           = EXCLUDED.status,
    cancellation_fee = EXCLUDED.cancellation_fee,
    updated_at       = now();

-- name: GetBookingById :one
SELECT * FROM bookings WHERE id = $1;

-- name: GetBookingByIdForUpdate :one
SELECT * FROM bookings WHERE id = $1
FOR UPDATE;

-- name: ListBookingsByBookerId :many
SELECT * FROM bookings
WHERE booker_id = $1
ORDER BY checkin_date DESC;
-- name: ListBookingSummariesByBookerId :many
SELECT id, room_type_id, checkin_date, checkout_date, total_fee, status, cancellation_fee
FROM bookings
WHERE booker_id = $1
ORDER BY checkin_date DESC;

-- name: ListStaleTemporaryHoldIds :many
SELECT id FROM bookings
WHERE status = 'temporary_hold' AND created_at < $1
ORDER BY created_at;
