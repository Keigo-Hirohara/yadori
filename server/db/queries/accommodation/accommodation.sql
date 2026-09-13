-- name: UpsertAccommodation :exec
INSERT INTO accommodations (
    id, name, phone_number, postal_code, prefecture, city, street_address, building, operator_subject
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9
)
ON CONFLICT (id) DO UPDATE SET
    name           = EXCLUDED.name,
    phone_number   = EXCLUDED.phone_number,
    postal_code    = EXCLUDED.postal_code,
    prefecture     = EXCLUDED.prefecture,
    city           = EXCLUDED.city,
    street_address = EXCLUDED.street_address,
    building       = EXCLUDED.building,
    updated_at     = now();

-- name: GetAccommodation :one
SELECT * FROM accommodations WHERE id = $1;
-- name: ListAccommodations :many
SELECT * FROM accommodations
ORDER BY created_at DESC;

-- name: ListAccommodationsByOperator :many
SELECT * FROM accommodations
WHERE operator_subject = $1
ORDER BY created_at DESC;

-- name: GetRoomTypeOperator :one
SELECT a.operator_subject
FROM room_types rt
JOIN accommodations a ON a.id = rt.accommodation_id
WHERE rt.id = $1 AND rt.deleted_at IS NULL;
