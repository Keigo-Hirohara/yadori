-- name: UpsertGuest :exec
INSERT INTO guests (
    id, booking_id, first_name, last_name
) VALUES (
    $1, $2, $3, $4
)
ON CONFLICT (id) DO UPDATE SET
    first_name = EXCLUDED.first_name,
    last_name  = EXCLUDED.last_name,
    updated_at = now();

-- name: ListGuestsByBookingId :many
SELECT * FROM guests
WHERE booking_id = $1
ORDER BY created_at;

-- name: DeleteGuestById :exec
DELETE FROM guests WHERE id = $1;

-- name: DeleteGuestsByBookingId :exec
DELETE FROM guests WHERE booking_id = $1;