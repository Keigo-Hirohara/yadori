package domain

import "errors"

var (
	ErrMinusFee = errors.New("マイナスの金額は設定できません")
)

type Fee struct {
	amount int
}

type CreateNewFeeInput struct {
	Amount int
}

func NewFee(input CreateNewFeeInput) (*Fee, error) {
	if err := validFee(input.Amount); err != nil {
		return nil, err
	}
	return &Fee{
		amount: input.Amount,
	}, nil
}

func (f *Fee) Amount() int {
	return f.amount
}

func validFee(amount int) error {
	if amount < 0 {
		return ErrMinusFee
	}
	return nil
}
