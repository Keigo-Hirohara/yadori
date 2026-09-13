-- name: SearchRoomTypes :many
WITH available AS (
    SELECT i.room_type_id, i.date, i.fee
    FROM inventories i
    LEFT JOIN holds h
        ON h.room_type_id = i.room_type_id AND h.date = i.date
    WHERE i.date >= @checkin_date
      AND i.date < @checkout_date
      AND i.is_closed = false
    GROUP BY i.room_type_id, i.date, i.fee, i.quantity_available
    HAVING i.quantity_available - COUNT(h.id) > 0
)
SELECT
    rt.id            AS room_type_id,
    rt.name          AS room_type_name,
    rt.capacity,
    rt.has_private_bath,
    rt.has_balcony,
    a.id             AS accommodation_id,
    a.name           AS accommodation_name,
    a.prefecture,
    a.city,
    SUM(av.fee)::int AS total_fee
FROM available av
JOIN room_types rt     ON rt.id = av.room_type_id
JOIN accommodations a  ON a.id = rt.accommodation_id
WHERE rt.deleted_at IS NULL
  AND rt.capacity >= @guests::int
  AND (@prefecture::text = '' OR a.prefecture = @prefecture::text)
GROUP BY rt.id, rt.name, rt.capacity, rt.has_private_bath, rt.has_balcony,
         a.id, a.name, a.prefecture, a.city
HAVING COUNT(av.date) = @nights::int
   AND (@max_fee::int = 0 OR SUM(av.fee) <= @max_fee::int)
ORDER BY SUM(av.fee);
