-- name: UpsertInventory :exec
INSERT INTO inventories (
    id, room_type_id, fee, quantity_available, is_closed, date
) VALUES (
    $1, $2, $3, $4, $5, $6,
)
ON CONFLICT (id) DO UPDATE SET
    fee = EXCLUDED.fee,
    quantity_available = EXCLUDED.quantity_available,
    is_closed = EXCLUDED.is_closed,
    date = EXCLUDED.date,
    updated_at     = now();

-- name: GetInventoriesById :one
SELECT * FROM inventories WHERE id = $1;

-- name: GetInventoriesByRoomTypeId :many
SELECT * FROM inventories WHERE room_type_id = $1;

-- name: CloseInventoryByRoomTypeIdAndDate :exec
UPDATE inventories SET is_closed = true WHERE room_type_id = $1 AND date = $2;

-- name: OpenInventoryByRoomTypeIdAndDate :exec
UPDATE inventories SET is_closed = false WHERE room_type_id = $1 AND date = $2;