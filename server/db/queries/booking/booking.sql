-- name: UpsertBooking :exec
INSERT INTO bookings (
    id, booker_id, room_type_id, total_fee, checkin_date, checkout_date, status
) VALUES (
    $1, $2, $3, $4, $5, $6, $7
)
ON CONFLICT (id) DO UPDATE SET
    total_fee     = EXCLUDED.total_fee,
    checkin_date  = EXCLUDED.checkin_date,
    checkout_date = EXCLUDED.checkout_date,
    status        = EXCLUDED.status,
    updated_at    = now();

-- name: GetBookingById :one
SELECT * FROM bookings WHERE id = $1;

-- name: GetBookingByIdForUpdate :one
SELECT * FROM bookings WHERE id = $1
FOR UPDATE;

-- name: ListBookingsByBookerId :many
SELECT * FROM bookings
WHERE booker_id = $1
ORDER BY checkin_date DESC;