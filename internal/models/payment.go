package models

import (
	"time"

	"github.com/shopspring/decimal"
)

type Payment struct {
	ClientId    int             `json:"clientId"`
	TrainerId   int             `json:"trainerId"`
	PaymentDate time.Time       `json:"paymentDate"`
	Amount      decimal.Decimal `json:"amount"`
}
