package domain

import "errors"

var (
	ErrInvalidTotalFee = errors.New("宿泊料金に負の値は設定できません")
)

type TotalFee struct {
	amount int
}

func NewTotalFee(amount int) (TotalFee, error) {
	if amount < 0 {
		return TotalFee{}, ErrInvalidTotalFee
	}
	return TotalFee{amount: amount}, nil
}

func (f TotalFee) Amount() int {
	return f.amount
}
