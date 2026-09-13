-- name: UpsertHold :exec
INSERT INTO holds (
    id, date, room_type_id, booking_id, status, expired_at, slot_no
) VALUES (
    $1, $2, $3, $4, $5, $6, $7
)
ON CONFLICT (id) DO UPDATE SET
    status = EXCLUDED.status,
    expired_at = EXCLUDED.expired_at,
    updated_at = now();

-- name: DeleteHoldById :exec
DELETE FROM holds WHERE id = $1;

-- name: ListExpiredHoldInventoryIds :many
SELECT DISTINCT room_type_id, date FROM holds
WHERE status = 'temporary_hold'
  AND expired_at IS NOT NULL
  AND expired_at < $1
ORDER BY date, room_type_id;

-- name: ListHoldsByRoomTypeIdAndDate :many
SELECT * FROM holds WHERE date = $1 AND room_type_id = $2;
