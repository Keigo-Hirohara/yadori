-- name: UpsertHold :exec
INSERT INTO holds (
    id, inventory_id, booking_id, status, expired_at, slot_no
) VALUES (
    $1, $2, $3, $4, $5, $6
)
ON CONFLICT (id) DO UPDATE SET
    status = EXCLUDED.status,
    expired_at = EXCLUDED.expired_at,
    updated_at = now();

-- name: DeleteHoldById :exec
DELETE FROM holds WHERE id = $1;

-- name: InsertHoldWithFreeSlot :one
INSERT INTO holds (id, inventory_id, booking_id, slot_no, status, expired_at)
SELECT
    sqlc.arg(hold_id),
    sqlc.arg(inventory_id),
    sqlc.arg(booking_id),
    s.slot_no,
    'temporary_hold',
    sqlc.arg(expired_at)
FROM generate_series(
    1,
    (SELECT quantity_available FROM inventories
     WHERE id = sqlc.arg(inventory_id) AND is_closed = false)
) AS s(slot_no)
WHERE NOT EXISTS (
    SELECT 1 FROM holds h
    WHERE h.inventory_id = sqlc.arg(inventory_id) AND h.slot_no = s.slot_no
)
ORDER BY s.slot_no
LIMIT 1
RETURNING *;