-- name: UpsertInventory :exec
INSERT INTO inventories (
    room_type_id, fee, quantity_available, is_closed, date
) VALUES (
    $1, $2, $3, $4, $5
)
ON CONFLICT (id) DO UPDATE SET
    fee = EXCLUDED.fee,
    quantity_available = EXCLUDED.quantity_available,
    is_closed = EXCLUDED.is_closed,
    updated_at     = now();

-- name: GetInventoriesByDateAndRoomTypeId :one
SELECT * FROM inventories WHERE date = $1 AND room_type_id = $2;

-- name: GetInventoriesByRoomTypeId :many
SELECT * FROM inventories WHERE room_type_id = $1;

-- name: GetInventoryByRoomTypeIdAndDateForUpdate :one
SELECT * FROM inventories
WHERE room_type_id = $1 AND date = $2
FOR UPDATE;