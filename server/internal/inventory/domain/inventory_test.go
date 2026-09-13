package domain

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

var (
	now          = time.Date(2026, 12, 20, 10, 0, 0, 0, time.UTC)
	fourDayLater = time.Date(2026, 12, 24, 0, 0, 0, 0, time.UTC)
)

func getThirtyMinutesLater() time.Time { return now.Add(30 * time.Minute) }

func newInventoryForTest(t *testing.T, quantity int) *Inventory {
	t.Helper()
	fee, err := NewFee(CreateNewFeeInput{
		Amount: 15000,
	})
	require.NoError(t, err)
	id, err := NewInventoryId(CreateNewInventoryIdInput{
		RoomTypeId: uuid.New(),
		Date:       fourDayLater,
		Now:        now,
	})
	require.NoError(t, err)
	inv, err := Register(id, quantity, fee)
	require.NoError(t, err)
	return inv
}

func addHold(t *testing.T, inv *Inventory) uuid.UUID {
	t.Helper()
	holdID := uuid.New()
	_, err := inv.Hold(HoldInput{
		HoldId:     uuid.New(),
		BookingId:  holdID,
		RoomTypeId: uuid.New(),
		ExpiredAt:  getThirtyMinutesLater(),
		Date:       now,
	})
	require.NoError(t, err)
	return holdID
}

func TestRegister(t *testing.T) {
	fee, _ := NewFee(CreateNewFeeInput{Amount: 15000})
	inventoryId, _ := NewInventoryId(CreateNewInventoryIdInput{
		RoomTypeId: uuid.New(),
		Date:       fourDayLater,
		Now:        now,
	})

	t.Run("販売可能数が1以上なら登録できる", func(t *testing.T) {
		inv, err := Register(inventoryId, 10, fee)
		require.NoError(t, err)
		require.Equal(t, 10, inv.QuantityAvailable())

		require.False(t, inv.IsClosed())
	})

	t.Run("販売可能数が0でも登録できる", func(t *testing.T) {
		inv, err := Register(inventoryId, 0, fee)
		require.NoError(t, err)
		require.Equal(t, 0, inv.Available())
	})

	t.Run("販売可能数が負なら登録できない", func(t *testing.T) {
		_, err := Register(inventoryId, -1, fee)
		require.ErrorIs(t, err, ErrInvalidQuantity)
	})
}

func TestInventory_Hold(t *testing.T) {
	t.Run("空きがあれば確保できる", func(t *testing.T) {
		inv := newInventoryForTest(t, 3)

		slotNo, err := inv.Hold(HoldInput{
			HoldId:     uuid.New(),
			BookingId:  uuid.New(),
			RoomTypeId: uuid.New(),
			ExpiredAt:  getThirtyMinutesLater(),
			Date:       now,
		})

		require.NoError(t, err)
		require.Equal(t, 1, slotNo)
		require.Equal(t, 1, inv.HoldCount())
		require.Equal(t, 2, inv.Available())
	})

	t.Run("販売可能数まで確保できる", func(t *testing.T) {
		inv := newInventoryForTest(t, 3)

		for i := 1; i <= 3; i++ {
			slotNo, err := inv.Hold(HoldInput{
				HoldId:     uuid.New(),
				BookingId:  uuid.New(),
				RoomTypeId: uuid.New(),
				ExpiredAt:  getThirtyMinutesLater(),
				Date:       now,
			})
			require.NoError(t, err)
			require.Equal(t, i, slotNo)
		}
		require.Equal(t, 0, inv.Available())
	})

	t.Run("満室のときは確保できない", func(t *testing.T) {
		inv := newInventoryForTest(t, 1)
		addHold(t, inv)

		_, err := inv.Hold(HoldInput{
			HoldId:     uuid.New(),
			BookingId:  uuid.New(),
			RoomTypeId: uuid.New(),
			ExpiredAt:  getThirtyMinutesLater(),
			Date:       now,
		})

		require.ErrorIs(t, err, ErrSoldOut)
		require.Equal(t, 1, inv.HoldCount())
	})

	t.Run("販売可能数が0なら確保できない", func(t *testing.T) {
		inv := newInventoryForTest(t, 0)

		_, err := inv.Hold(HoldInput{
			HoldId:     uuid.New(),
			BookingId:  uuid.New(),
			RoomTypeId: uuid.New(),
			ExpiredAt:  getThirtyMinutesLater(),
			Date:       now,
		})

		require.ErrorIs(t, err, ErrSoldOut)
	})

	t.Run("販売停止中は確保できない", func(t *testing.T) {
		inv := newInventoryForTest(t, 3)
		require.NoError(t, inv.Close())

		_, err := inv.Hold(HoldInput{
			HoldId:     uuid.New(),
			BookingId:  uuid.New(),
			RoomTypeId: uuid.New(),
			ExpiredAt:  getThirtyMinutesLater(),
			Date:       now,
		})
		require.ErrorIs(t, err, ErrClosed)
		require.Equal(t, 0, inv.HoldCount())
	})

	t.Run("同じ確保IDでは二重に確保できない", func(t *testing.T) {
		inv := newInventoryForTest(t, 3)
		holdId := addHold(t, inv)

		_, err := inv.Hold(HoldInput{
			HoldId:     uuid.New(),
			BookingId:  holdId,
			RoomTypeId: uuid.New(),
			ExpiredAt:  getThirtyMinutesLater(),
			Date:       now,
		})

		require.ErrorIs(t, err, ErrDuplicatedHold)
		require.Equal(t, 1, inv.HoldCount())
	})

	t.Run("期限が現在より前なら確保できない", func(t *testing.T) {
		inv := newInventoryForTest(t, 3)

		_, err := inv.Hold(HoldInput{
			HoldId:     uuid.New(),
			BookingId:  uuid.New(),
			RoomTypeId: uuid.New(),
			ExpiredAt:  now.Add(-time.Minute),
			Date:       now,
		})

		require.ErrorIs(t, err, ErrInvalidExpiration)
	})

	t.Run("期限が現在と同時刻なら確保できない", func(t *testing.T) {
		inv := newInventoryForTest(t, 3)

		_, err := inv.Hold(HoldInput{
			HoldId:     uuid.New(),
			BookingId:  uuid.New(),
			RoomTypeId: uuid.New(),
			ExpiredAt:  now,
			Date:       now,
		})

		require.ErrorIs(t, err, ErrInvalidExpiration)
	})

	t.Run("確保IDは渡されたものがそのまま使われる", func(t *testing.T) {
		inv := newInventoryForTest(t, 3)
		holdId := uuid.New()
		bookingId := uuid.New()

		_, err := inv.Hold(HoldInput{
			HoldId:     holdId,
			BookingId:  bookingId,
			RoomTypeId: uuid.New(),
			ExpiredAt:  getThirtyMinutesLater(),
			Date:       now,
		})

		require.NoError(t, err)
		h, ok := inv.FindHold(bookingId)
		require.True(t, ok)
		require.Equal(t, holdId, h.Id())
	})

	t.Run("解放された枠番号は再利用される", func(t *testing.T) {
		inv := newInventoryForTest(t, 3)
		h1 := addHold(t, inv)
		addHold(t, inv)
		require.NoError(t, inv.Release(h1))

		slotNo, err := inv.Hold(HoldInput{
			HoldId:     uuid.New(),
			BookingId:  uuid.New(),
			RoomTypeId: uuid.New(),
			ExpiredAt:  getThirtyMinutesLater(),
			Date:       now,
		})

		require.NoError(t, err)
		require.Equal(t, 1, slotNo)
	})
}

func TestInventory_StartPayment(t *testing.T) {
	t.Run("仮確保を決済中にできる", func(t *testing.T) {
		inv := newInventoryForTest(t, 3)
		holdID := addHold(t, inv)

		err := inv.StartPayment(holdID)

		require.NoError(t, err)
		h, ok := inv.FindHold(holdID)
		require.True(t, ok)
		require.Equal(t, ProcessingPayment, h.Status())
	})

	t.Run("決済中にすると期限が外れる", func(t *testing.T) {
		inv := newInventoryForTest(t, 3)
		holdID := addHold(t, inv)

		require.NoError(t, inv.StartPayment(holdID))

		h, _ := inv.FindHold(holdID)
		require.Nil(t, h.ExpiredAt())
	})

	t.Run("存在しない確保IDではエラー", func(t *testing.T) {
		inv := newInventoryForTest(t, 3)

		err := inv.StartPayment(uuid.New())

		require.ErrorIs(t, err, ErrHoldNotFound)
	})

	t.Run("確定済みからは決済中にできない", func(t *testing.T) {
		inv := newInventoryForTest(t, 3)
		holdID := addHold(t, inv)
		require.NoError(t, inv.StartPayment(holdID))
		require.NoError(t, inv.Confirm(holdID))

		err := inv.StartPayment(holdID)

		require.ErrorIs(t, err, ErrInvalidTransition)
	})
}

func TestInventory_Confirm(t *testing.T) {
	t.Run("決済中を確定にできる", func(t *testing.T) {
		inv := newInventoryForTest(t, 3)
		holdID := addHold(t, inv)
		require.NoError(t, inv.StartPayment(holdID))

		err := inv.Confirm(holdID)

		require.NoError(t, err)
		h, _ := inv.FindHold(holdID)
		require.Equal(t, Confirmed, h.Status())
		require.Nil(t, h.ExpiredAt())
	})

	t.Run("仮確保から直接確定にはできない", func(t *testing.T) {
		inv := newInventoryForTest(t, 3)
		holdID := addHold(t, inv)

		err := inv.Confirm(holdID)

		require.ErrorIs(t, err, ErrInvalidTransition)
	})

	t.Run("存在しない確保IDではエラー", func(t *testing.T) {
		inv := newInventoryForTest(t, 3)

		err := inv.Confirm(uuid.New())

		require.ErrorIs(t, err, ErrHoldNotFound)
	})
}

func TestInventory_Release(t *testing.T) {
	t.Run("確保を解放できる", func(t *testing.T) {
		inv := newInventoryForTest(t, 3)
		holdID := addHold(t, inv)

		err := inv.Release(holdID)

		require.NoError(t, err)
		require.Equal(t, 0, inv.HoldCount())
		require.Equal(t, 3, inv.Available())
		_, ok := inv.FindHold(holdID)
		require.False(t, ok)
	})

	t.Run("確定済みも解放できる", func(t *testing.T) {
		inv := newInventoryForTest(t, 3)
		holdID := addHold(t, inv)
		require.NoError(t, inv.StartPayment(holdID))
		require.NoError(t, inv.Confirm(holdID))

		err := inv.Release(holdID)

		require.NoError(t, err)
		require.Equal(t, 0, inv.HoldCount())
	})

	t.Run("存在しない確保IDではエラー", func(t *testing.T) {
		inv := newInventoryForTest(t, 3)

		err := inv.Release(uuid.New())

		require.ErrorIs(t, err, ErrHoldNotFound)
	})
}

func TestInventory_CollectExpired(t *testing.T) {
	t.Run("期限切れの仮確保が回収される", func(t *testing.T) {
		inv := newInventoryForTest(t, 3)
		addHold(t, inv)

		collected := inv.CollectExpired(getThirtyMinutesLater().Add(time.Second))

		require.Equal(t, 1, collected)
		require.Equal(t, 0, inv.HoldCount())
		require.Equal(t, 3, inv.Available())
	})

	t.Run("期限内の仮確保は回収されない", func(t *testing.T) {
		inv := newInventoryForTest(t, 3)
		addHold(t, inv)

		collected := inv.CollectExpired(now.Add(time.Minute))

		require.Equal(t, 0, collected)
		require.Equal(t, 1, inv.HoldCount())
	})

	t.Run("期限ちょうどでは回収されない", func(t *testing.T) {
		inv := newInventoryForTest(t, 3)
		addHold(t, inv)

		collected := inv.CollectExpired(getThirtyMinutesLater())

		require.Equal(t, 0, collected)
	})

	t.Run("決済中の確保は回収されない", func(t *testing.T) {
		inv := newInventoryForTest(t, 3)
		holdID := addHold(t, inv)
		require.NoError(t, inv.StartPayment(holdID))

		collected := inv.CollectExpired(getThirtyMinutesLater().Add(24 * time.Hour))

		require.Equal(t, 0, collected)
		require.Equal(t, 1, inv.HoldCount())
	})

	t.Run("確定済みの確保は回収されない", func(t *testing.T) {
		inv := newInventoryForTest(t, 3)
		holdID := addHold(t, inv)
		require.NoError(t, inv.StartPayment(holdID))
		require.NoError(t, inv.Confirm(holdID))

		collected := inv.CollectExpired(getThirtyMinutesLater().Add(24 * time.Hour))

		require.Equal(t, 0, collected)
		require.Equal(t, 1, inv.HoldCount())
	})

	t.Run("期限切れのものだけが回収される", func(t *testing.T) {
		inv := newInventoryForTest(t, 3)
		expiredBookingId := uuid.New()
		_, err := inv.Hold(HoldInput{
			HoldId:     uuid.New(),
			BookingId:  expiredBookingId,
			RoomTypeId: uuid.New(),
			ExpiredAt:  now.Add(10 * time.Minute),
			Date:       now,
		})
		require.NoError(t, err)
		unexpiredBookingId := uuid.New()
		_, err = inv.Hold(HoldInput{
			HoldId:     uuid.New(),
			BookingId:  unexpiredBookingId,
			RoomTypeId: uuid.New(),
			ExpiredAt:  now.Add(60 * time.Minute),
			Date:       now,
		})
		require.NoError(t, err)

		collected := inv.CollectExpired(now.Add(30 * time.Minute))

		require.Equal(t, 1, collected)
		require.Equal(t, 1, inv.HoldCount())
		_, ok := inv.FindHold(unexpiredBookingId)
		require.True(t, ok)
	})
}

func TestInventory_ChangeQuantity(t *testing.T) {
	t.Run("販売可能数を増やせる", func(t *testing.T) {
		inv := newInventoryForTest(t, 3)

		err := inv.ChangeQuantity(5)

		require.NoError(t, err)
		require.Equal(t, 5, inv.QuantityAvailable())
	})

	t.Run("確保数と同じ値には変更できる", func(t *testing.T) {
		inv := newInventoryForTest(t, 3)
		addHold(t, inv)
		addHold(t, inv)

		err := inv.ChangeQuantity(2)

		require.NoError(t, err)
		require.Equal(t, 0, inv.Available())
	})

	t.Run("確保数より少なくはできない", func(t *testing.T) {
		inv := newInventoryForTest(t, 3)
		addHold(t, inv)
		addHold(t, inv)

		err := inv.ChangeQuantity(1)

		require.ErrorIs(t, err, ErrQuantityBelowHolds)
		require.Equal(t, 3, inv.QuantityAvailable())
	})

	t.Run("負の値には変更できない", func(t *testing.T) {
		inv := newInventoryForTest(t, 3)

		err := inv.ChangeQuantity(-1)

		require.ErrorIs(t, err, ErrInvalidQuantity)
	})
}

func TestInventory_Close(t *testing.T) {
	t.Run("販売を停止できる", func(t *testing.T) {
		inv := newInventoryForTest(t, 3)

		err := inv.Close()

		require.NoError(t, err)
		require.True(t, inv.IsClosed())
	})

	t.Run("停止しても既存の確保は残る", func(t *testing.T) {
		inv := newInventoryForTest(t, 3)
		addHold(t, inv)

		require.NoError(t, inv.Close())

		require.Equal(t, 1, inv.HoldCount())
		require.Equal(t, 3, inv.QuantityAvailable())
	})

	t.Run("既に停止中なら停止できない", func(t *testing.T) {
		inv := newInventoryForTest(t, 3)
		require.NoError(t, inv.Close())

		err := inv.Close()

		require.ErrorIs(t, err, ErrAlreadyClosed)
	})
}

func TestInventory_Reopen(t *testing.T) {
	t.Run("販売を再開できる", func(t *testing.T) {
		inv := newInventoryForTest(t, 3)
		require.NoError(t, inv.Close())

		err := inv.Reopen()

		require.NoError(t, err)
		require.False(t, inv.IsClosed())
	})

	t.Run("再開後は確保できる", func(t *testing.T) {
		inv := newInventoryForTest(t, 3)
		require.NoError(t, inv.Close())
		require.NoError(t, inv.Reopen())

		_, err := inv.Hold(HoldInput{
			HoldId:     uuid.New(),
			BookingId:  uuid.New(),
			RoomTypeId: uuid.New(),
			ExpiredAt:  getThirtyMinutesLater(),
			Date:       now,
		})

		require.NoError(t, err)
	})

	t.Run("停止していなければ再開できない", func(t *testing.T) {
		inv := newInventoryForTest(t, 3)

		err := inv.Reopen()

		require.ErrorIs(t, err, ErrNotClosed)
	})
}

func TestInventory_ChangeFee(t *testing.T) {
	t.Run("料金を変更できる", func(t *testing.T) {
		inv := newInventoryForTest(t, 3)
		newFee, _ := NewFee(CreateNewFeeInput{
			Amount: 20000,
		})

		err := inv.ChangeFee(newFee)

		require.NoError(t, err)
		require.Equal(t, 20000, inv.Fee().Amount())
	})

	t.Run("確保があっても料金を変更できる", func(t *testing.T) {
		inv := newInventoryForTest(t, 3)
		addHold(t, inv)
		newFee, _ := NewFee(CreateNewFeeInput{
			Amount: 20000,
		})

		err := inv.ChangeFee(newFee)

		require.NoError(t, err)
	})
}
