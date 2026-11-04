package domain

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

var processStart = time.Date(2026, 12, 20, 10, 0, 0, 0, time.UTC)

func processIn(t *testing.T, state ProcessState) *BookingProcess {
	t.Helper()
	p := StartBookingProcess(uuid.New(), processStart)
	steps := []struct {
		reached ProcessState
		move    func() error
	}{
		{AwaitingPayment, func() error { return p.FinishHolding(processStart) }},
		{PreparingPayment, func() error { return p.PreparePayment(processStart) }},
		{PaymentRequested, func() error { return p.RequestPayment(uuid.New(), processStart) }},
		{Confirming, func() error { return p.PaymentSucceeded(processStart) }},
		{Completed, func() error { return p.FinishConfirming(processStart) }},
	}
	for _, s := range steps {
		if p.State() == state {
			return p
		}
		require.NoError(t, s.move())
		require.Equal(t, s.reached, p.State())
	}
	require.Equal(t, state, p.State())
	return p
}

func releasingProcess(t *testing.T) *BookingProcess {
	t.Helper()
	p := processIn(t, AwaitingPayment)
	require.NoError(t, p.Release(Expired, processStart))
	return p
}

func releasedProcess(t *testing.T) *BookingProcess {
	t.Helper()
	p := releasingProcess(t)
	require.NoError(t, p.FinishReleasing(processStart))
	return p
}

func TestStartBookingProcess(t *testing.T) {
	t.Run("確保中から始まる", func(t *testing.T) {
		bookingId := uuid.New()

		p := StartBookingProcess(bookingId, processStart)

		require.Equal(t, bookingId, p.BookingId())
		require.Equal(t, Holding, p.State())
		require.Nil(t, p.PaymentId())
		require.Nil(t, p.ReleaseReason())
		require.Equal(t, processStart, p.StartedAt())
	})
}

func TestBookingProcess_FinishHolding(t *testing.T) {
	t.Run("確保が終われば支払い待ちになる", func(t *testing.T) {
		p := processIn(t, Holding)
		later := processStart.Add(time.Second)

		err := p.FinishHolding(later)

		require.NoError(t, err)
		require.Equal(t, AwaitingPayment, p.State())
		require.Equal(t, later, p.UpdatedAt())
	})

	t.Run("支払い待ちからは確保を終えられない", func(t *testing.T) {
		p := processIn(t, AwaitingPayment)

		err := p.FinishHolding(processStart)

		require.ErrorIs(t, err, ErrInvalidTransition)
	})
}

func TestBookingProcess_PreparePayment(t *testing.T) {
	t.Run("支払い待ちなら決済の準備に進める", func(t *testing.T) {
		p := processIn(t, AwaitingPayment)

		err := p.PreparePayment(processStart.Add(10 * time.Minute))

		require.NoError(t, err)
		require.Equal(t, PreparingPayment, p.State())
	})

	t.Run("支払いの期限を過ぎたら決済の準備に進めない", func(t *testing.T) {
		p := processIn(t, AwaitingPayment)

		err := p.PreparePayment(processStart.Add(PaymentWindow))

		require.ErrorIs(t, err, ErrPaymentDeadlinePassed)
		require.Equal(t, AwaitingPayment, p.State())
	})

	t.Run("確保中からは決済の準備に進めない", func(t *testing.T) {
		p := processIn(t, Holding)

		err := p.PreparePayment(processStart)

		require.ErrorIs(t, err, ErrInvalidTransition)
	})
}

func TestBookingProcess_RequestPayment(t *testing.T) {
	t.Run("決済の準備中なら、依頼IDを記録して決済依頼済みになる", func(t *testing.T) {
		p := processIn(t, PreparingPayment)
		paymentId := uuid.New()

		err := p.RequestPayment(paymentId, processStart)

		require.NoError(t, err)
		require.Equal(t, PaymentRequested, p.State())
		require.Equal(t, paymentId, *p.PaymentId())
	})

	t.Run("支払い待ちから直接決済を依頼できない", func(t *testing.T) {
		p := processIn(t, AwaitingPayment)

		err := p.RequestPayment(uuid.New(), processStart)

		require.ErrorIs(t, err, ErrInvalidTransition)
		require.Nil(t, p.PaymentId())
	})
}

func TestBookingProcess_PaymentSucceeded(t *testing.T) {
	t.Run("決済依頼済みなら確定中になる", func(t *testing.T) {
		p := processIn(t, PaymentRequested)

		err := p.PaymentSucceeded(processStart)

		require.NoError(t, err)
		require.Equal(t, Confirming, p.State())
	})

	t.Run("決済を依頼していなければ成功にできない", func(t *testing.T) {
		p := processIn(t, PreparingPayment)

		err := p.PaymentSucceeded(processStart)

		require.ErrorIs(t, err, ErrInvalidTransition)
	})
}

func TestBookingProcess_PaymentFailed(t *testing.T) {
	t.Run("決済依頼済みなら、決済失敗を理由に解放中になる", func(t *testing.T) {
		p := processIn(t, PaymentRequested)

		err := p.PaymentFailed(processStart)

		require.NoError(t, err)
		require.Equal(t, Releasing, p.State())
		require.Equal(t, PaymentFailed, *p.ReleaseReason())
	})

	t.Run("確定中からは失敗にできない", func(t *testing.T) {
		p := processIn(t, Confirming)

		err := p.PaymentFailed(processStart)

		require.ErrorIs(t, err, ErrInvalidTransition)
	})
}

func TestBookingProcess_FinishConfirming(t *testing.T) {
	t.Run("確定中なら完了になる", func(t *testing.T) {
		p := processIn(t, Confirming)

		err := p.FinishConfirming(processStart)

		require.NoError(t, err)
		require.Equal(t, Completed, p.State())
	})

	t.Run("決済依頼済みから直接完了にはできない", func(t *testing.T) {
		p := processIn(t, PaymentRequested)

		err := p.FinishConfirming(processStart)

		require.ErrorIs(t, err, ErrInvalidTransition)
	})
}

func TestBookingProcess_Release(t *testing.T) {
	for _, state := range []ProcessState{Holding, AwaitingPayment, PreparingPayment} {
		t.Run(state.String()+"なら解放に進める", func(t *testing.T) {
			p := processIn(t, state)

			err := p.Release(Expired, processStart)

			require.NoError(t, err)
			require.Equal(t, Releasing, p.State())
			require.Equal(t, Expired, *p.ReleaseReason())
		})
	}

	t.Run("決済を依頼したあとは解放に進めない", func(t *testing.T) {
		p := processIn(t, PaymentRequested)

		err := p.Release(Expired, processStart)

		require.ErrorIs(t, err, ErrInvalidTransition)
		require.Nil(t, p.ReleaseReason())
	})

	t.Run("確定中は解放に進めない", func(t *testing.T) {
		p := processIn(t, Confirming)

		err := p.Release(Expired, processStart)

		require.ErrorIs(t, err, ErrInvalidTransition)
	})

	t.Run("完了したあとは解放に進めない", func(t *testing.T) {
		p := processIn(t, Completed)

		err := p.Release(ByGuest, processStart)

		require.ErrorIs(t, err, ErrInvalidTransition)
	})
}

func TestBookingProcess_FinishReleasing(t *testing.T) {
	t.Run("解放中なら取り消し済みになり、理由は残る", func(t *testing.T) {
		p := releasingProcess(t)

		err := p.FinishReleasing(processStart)

		require.NoError(t, err)
		require.Equal(t, Released, p.State())
		require.Equal(t, Expired, *p.ReleaseReason())
	})

	t.Run("支払い待ちから直接取り消し済みにはできない", func(t *testing.T) {
		p := processIn(t, AwaitingPayment)

		err := p.FinishReleasing(processStart)

		require.ErrorIs(t, err, ErrInvalidTransition)
	})
}

func TestBookingProcess_NextResumeAction(t *testing.T) {
	stalled := processStart.Add(StallAfter)
	fresh := processStart.Add(StallAfter - time.Second)

	cases := []struct {
		name    string
		process func(t *testing.T) *BookingProcess
		now     time.Time
		want    ResumeAction
	}{
		{"確保中で止まったら期限切れとして解放する", func(t *testing.T) *BookingProcess { return processIn(t, Holding) }, stalled, ReleaseAsExpired},
		{"決済の準備中で止まったら期限切れとして解放する", func(t *testing.T) *BookingProcess { return processIn(t, PreparingPayment) }, stalled, ReleaseAsExpired},
		{"決済依頼済みで止まったら決済サービスに照合する", func(t *testing.T) *BookingProcess { return processIn(t, PaymentRequested) }, stalled, InquirePayment},
		{"確定中で止まったら続きから確定する", func(t *testing.T) *BookingProcess { return processIn(t, Confirming) }, stalled, ContinueConfirming},
		{"解放中で止まったら続きから解放する", releasingProcess, stalled, ContinueReleasing},
		{"更新されたばかりなら何もしない", func(t *testing.T) *BookingProcess { return processIn(t, Confirming) }, fresh, NothingToDo},
		{"支払い待ちは期限前なら止まって見えても何もしない", func(t *testing.T) *BookingProcess { return processIn(t, AwaitingPayment) }, processStart.Add(PaymentWindow - time.Second), NothingToDo},
		{"支払い待ちは期限を過ぎたら期限切れとして解放する", func(t *testing.T) *BookingProcess { return processIn(t, AwaitingPayment) }, processStart.Add(PaymentWindow), ReleaseAsExpired},
		{"完了したら何もしない", func(t *testing.T) *BookingProcess { return processIn(t, Completed) }, stalled, NothingToDo},
		{"取り消し済みなら何もしない", releasedProcess, stalled, NothingToDo},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			p := c.process(t)

			got := p.NextResumeAction(c.now)

			require.Equal(t, c.want, got)
		})
	}
}
