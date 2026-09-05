-- name: UpsertInventory :exec
INSERT INTO inventories (
    id, room_type_id, fee, quantity_available, is_closed, date
) VALUES (
    $1, $2, $3, $4, $5, $6
)
ON CONFLICT (id) DO UPDATE SET
    fee = EXCLUDED.fee,
    quantity_available = EXCLUDED.quantity_available,
    is_closed = EXCLUDED.is_closed,
    updated_at     = now();

-- name: GetInventoriesById :one
SELECT * FROM inventories WHERE id = $1;

-- name: GetInventoriesByRoomTypeId :many
SELECT * FROM inventories WHERE room_type_id = $1;

-- name: OpenInventoryByRoomTypeIdAndDate :exec
UPDATE inventories SET is_closed = false WHERE room_type_id = $1 AND date = $2;

-- name: ListHoldsByInventoryId :many
SELECT * FROM holds WHERE inventory_id = $1 ORDER BY slot_no;

-- name: GetInventoryByRoomTypeIdAndDateForUpdate :one
SELECT * FROM inventories
WHERE room_type_id = $1 AND date = $2
FOR UPDATE;