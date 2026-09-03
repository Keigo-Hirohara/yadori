-- name: UpsertRoomType :exec
INSERT INTO room_types (
    id, accommodation_id, name, capacity, has_private_bath, has_balcony
) VALUES (
    $1, $2, $3, $4, $5, $6
)
ON CONFLICT (id) DO UPDATE SET
    name             = EXCLUDED.name,
    capacity         = EXCLUDED.capacity,
    has_private_bath = EXCLUDED.has_private_bath,
    has_balcony      = EXCLUDED.has_balcony,
    updated_at       = now();

-- name: GetRoomType :one
SELECT * FROM room_types
WHERE id = $1 AND deleted_at IS NULL;

-- name: ListRoomTypesByAccommodation :many
SELECT * FROM room_types
WHERE accommodation_id = $1 AND deleted_at IS NULL
ORDER BY created_at;

-- name: SoftDeleteRoomType :exec
UPDATE room_types
SET deleted_at = now(), updated_at = now()
WHERE id = $1 AND deleted_at IS NULL;