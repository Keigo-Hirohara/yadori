-- name: UpsertBooker :exec
INSERT INTO bookers (
    id, first_name, last_name, postal_code, phone_number,
    prefecture, city, street_address, building
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9
)
ON CONFLICT (id) DO UPDATE SET
    first_name     = EXCLUDED.first_name,
    last_name      = EXCLUDED.last_name,
    postal_code    = EXCLUDED.postal_code,
    phone_number   = EXCLUDED.phone_number,
    prefecture     = EXCLUDED.prefecture,
    city           = EXCLUDED.city,
    street_address = EXCLUDED.street_address,
    building       = EXCLUDED.building,
    updated_at     = now();

-- name: GetBooker :one
SELECT * FROM bookers WHERE id = $1;