-- name: UpsertInventory :exec
INSERT INTO inventories (
    room_type_id, fee, quantity_available, is_closed, date
) VALUES (
    $1, $2, $3, $4, $5
)
ON CONFLICT (room_type_id, date) DO UPDATE SET
    fee = EXCLUDED.fee,
    quantity_available = EXCLUDED.quantity_available,
    is_closed = EXCLUDED.is_closed,
    updated_at     = now();

-- name: InsertInventory :exec
INSERT INTO inventories (
    room_type_id, date, fee, quantity_available, is_closed
) VALUES (
    $1, $2, $3, $4, $5
);

-- name: GetInventoriesByDateAndRoomTypeId :one
SELECT * FROM inventories WHERE date = $1 AND room_type_id = $2;

-- name: GetInventoriesByRoomTypeId :many
SELECT * FROM inventories WHERE room_type_id = $1;

-- name: GetInventoryByRoomTypeIdAndDateForUpdate :one
SELECT * FROM inventories
WHERE room_type_id = $1 AND date = $2
FOR UPDATE;
-- name: ListInventorySummaries :many
SELECT
    i.room_type_id,
    i.date,
    i.fee,
    i.quantity_available,
    i.is_closed,
    COUNT(h.id)::int AS held_count
FROM inventories i
LEFT JOIN holds h
    ON h.room_type_id = i.room_type_id AND h.date = i.date
WHERE i.room_type_id = $1
  AND i.date >= $2
  AND i.date < $3
GROUP BY i.room_type_id, i.date, i.fee, i.quantity_available, i.is_closed
ORDER BY i.date;
