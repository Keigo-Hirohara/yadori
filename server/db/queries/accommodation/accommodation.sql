-- name: UpsertAccommodation :exec
INSERT INTO accommodations (
    id, name, phone_number, postal_code, prefecture, city, street_address, building
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8
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