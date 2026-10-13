package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

const marker = "loadtest"

const insertAccommodations = `
INSERT INTO accommodations (
    id, name, phone_number, postal_code,
    prefecture, city, street_address, operator_subject
)
SELECT
    gen_random_uuid(),
    '負荷試験の宿 ' || i::text,
    '0312345678',
    '1000001',
    (ARRAY['北海道','青森県','岩手県','宮城県','秋田県','山形県','福島県',
           '茨城県','栃木県','群馬県','埼玉県','千葉県','東京都','神奈川県',
           '新潟県','富山県','石川県','福井県','山梨県','長野県',
           '岐阜県','静岡県','愛知県','三重県',
           '滋賀県','京都府','大阪府','兵庫県','奈良県','和歌山県',
           '鳥取県','島根県','岡山県','広島県','山口県',
           '徳島県','香川県','愛媛県','高知県',
           '福岡県','佐賀県','長崎県','熊本県','大分県','宮崎県','鹿児島県','沖縄県'])[1 + (i - 1) % 47],
    (ARRAY['札幌市','青森市','盛岡市','仙台市','秋田市','山形市','福島市',
           '水戸市','宇都宮市','前橋市','さいたま市','千葉市','新宿区','横浜市',
           '新潟市','富山市','金沢市','福井市','甲府市','長野市',
           '岐阜市','静岡市','名古屋市','津市',
           '大津市','京都市','大阪市','神戸市','奈良市','和歌山市',
           '鳥取市','松江市','岡山市','広島市','山口市',
           '徳島市','高松市','松山市','高知市',
           '福岡市','佐賀市','長崎市','熊本市','大分市','宮崎市','鹿児島市','那覇市'])[1 + (i - 1) % 47],
    '1-1-1',
    $2
FROM generate_series(1, $1) AS i`

const insertRoomTypes = `
INSERT INTO room_types (
    id, accommodation_id, name, capacity, has_private_bath, has_balcony
)
SELECT
    gen_random_uuid(),
    a.id,
    (ARRAY['シングル','ツイン','和室10畳'])[1 + (j - 1) % 3],
    (ARRAY[1, 2, 4])[1 + (j - 1) % 3],
    j % 3 = 0,
    j % 2 = 0
FROM accommodations a
CROSS JOIN generate_series(1, $1) AS j
WHERE a.operator_subject = $2`

const insertInventories = `
INSERT INTO inventories (
    room_type_id, date, fee, quantity_available, is_closed
)
SELECT
    rt.id,
    d::date,
    1000 * (8 + (random() * 24)::int),
    3 + (random() * 7)::int,
    random() < 0.02
FROM room_types rt
JOIN accommodations a ON a.id = rt.accommodation_id
CROSS JOIN generate_series($1::date, $1::date + ($2 - 1), interval '1 day') AS d
WHERE a.operator_subject = $3`

const insertBookers = `
INSERT INTO bookers (
    id, first_name, last_name, postal_code, phone_number,
    prefecture, city, street_address, subject
)
SELECT
    ('c0000000-0000-4000-8000-' || lpad(i::text, 12, '0'))::uuid,
    '試験',
    '利用者' || i::text,
    '1000001',
    '09012345678',
    '東京都',
    '千代田区',
    '1-1-1',
    $2 || '-booker-' || i::text
FROM generate_series(1, $1) AS i`

const holdDays = 180

const insertBookings = `
WITH target AS MATERIALIZED (
    SELECT
        i.room_type_id,
        i.date,
        i.fee,
        (random() * i.quantity_available)::int AS held
    FROM inventories i
    JOIN room_types rt ON rt.id = i.room_type_id
    JOIN accommodations a ON a.id = rt.accommodation_id
    WHERE a.operator_subject = $3
      AND i.date < $1::date + $2::int
)
INSERT INTO bookings (
    id, booker_id, room_type_id, total_fee,
    checkin_date, checkout_date, status
)
SELECT
    gen_random_uuid(),
    ('c0000000-0000-4000-8000-' || lpad((1 + (random() * ($4 - 1))::int)::text, 12, '0'))::uuid,
    t.room_type_id,
    t.fee,
    t.date,
    t.date + 1,
    'confirmed'
FROM target t
CROSS JOIN generate_series(1, t.held) AS s`

const insertHolds = `
INSERT INTO holds (
    id, room_type_id, date, booking_id, status, expired_at, slot_no
)
SELECT
    gen_random_uuid(),
    b.room_type_id,
    b.checkin_date,
    b.id,
    'confirmed',
    NULL,
    row_number() OVER (PARTITION BY b.room_type_id, b.checkin_date ORDER BY b.id)
FROM bookings b
JOIN room_types rt ON rt.id = b.room_type_id
JOIN accommodations a ON a.id = rt.accommodation_id
WHERE a.operator_subject = $1`

const insertGuests = `
INSERT INTO guests (id, booking_id, first_name, last_name)
SELECT gen_random_uuid(), b.id, '試験', '宿泊者'
FROM bookings b
JOIN room_types rt ON rt.id = b.room_type_id
JOIN accommodations a ON a.id = rt.accommodation_id
WHERE a.operator_subject = $1`

var deleteStatements = []string{
	`DELETE FROM holds WHERE room_type_id IN (
        SELECT rt.id FROM room_types rt
        JOIN accommodations a ON a.id = rt.accommodation_id
        WHERE a.operator_subject = $1
    )`,
	`DELETE FROM bookings WHERE room_type_id IN (
        SELECT rt.id FROM room_types rt
        JOIN accommodations a ON a.id = rt.accommodation_id
        WHERE a.operator_subject = $1
    )`,
	`DELETE FROM inventories WHERE room_type_id IN (
        SELECT rt.id FROM room_types rt
        JOIN accommodations a ON a.id = rt.accommodation_id
        WHERE a.operator_subject = $1
    )`,
	`DELETE FROM room_types WHERE accommodation_id IN (
        SELECT id FROM accommodations WHERE operator_subject = $1
    )`,
	`DELETE FROM accommodations WHERE operator_subject = $1`,
	`DELETE FROM bookers WHERE subject LIKE $1 || '-booker-%'`,
}

func main() {
	accommodations := flag.Int("accommodations", 300, "宿の件数")
	roomTypes := flag.Int("room-types", 3, "1つの宿あたりの部屋タイプ数")
	days := flag.Int("days", 730, "今日から何日分の在庫を作るか")
	bookers := flag.Int("bookers", 2000, "会員の件数")
	flag.Parse()

	if err := run(*accommodations, *roomTypes, *days, *bookers); err != nil {
		log.Fatal(err)
	}
}

func exec(ctx context.Context, pool *pgxpool.Pool, label, sql string, args ...any) error {
	tag, err := pool.Exec(ctx, sql, args...)
	if err != nil {
		return fmt.Errorf("%s の投入に失敗しました: %w", label, err)
	}
	fmt.Printf("%-12s %9d 行\n", label, tag.RowsAffected())
	return nil
}

func run(accommodations, roomTypes, days, bookers int) error {
	ctx := context.Background()

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		return errors.New("DATABASE_URL が設定されていません")
	}

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return err
	}
	defer pool.Close()

	for _, sql := range deleteStatements {
		if err := exec(ctx, pool, "削除", sql, marker); err != nil {
			return err
		}
	}

	if err := exec(ctx, pool, "宿", insertAccommodations, accommodations, marker); err != nil {
		return err
	}
	if err := exec(ctx, pool, "部屋タイプ", insertRoomTypes, roomTypes, marker); err != nil {
		return err
	}
	today := time.Now().Format("2006-01-02")
	if err := exec(ctx, pool, "在庫", insertInventories, today, days, marker); err != nil {
		return err
	}
	if err := exec(ctx, pool, "会員", insertBookers, bookers, marker); err != nil {
		return err
	}
	if err := exec(ctx, pool, "予約", insertBookings, today, holdDays, marker, bookers); err != nil {
		return err
	}
	if err := exec(ctx, pool, "確保", insertHolds, marker); err != nil {
		return err
	}
	if err := exec(ctx, pool, "宿泊者", insertGuests, marker); err != nil {
		return err
	}
	return nil
}
