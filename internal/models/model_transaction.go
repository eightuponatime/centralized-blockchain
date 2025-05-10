package models

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"time"

	"github.com/shopspring/decimal"
)

type Transaction struct {
	TrainerId   int             `json:"trainerId"`
	ClientId    int             `json:"clientId"`
	PaymentDate time.Time       `json:"paymentDate"`
	Amount      decimal.Decimal `json:"amount"`
}

func (t *Transaction) SerializeTransaction() ([]byte, error) {

	var buf bytes.Buffer
	data := new(bytes.Buffer)
	if err := binary.Write(data, binary.BigEndian, int32(t.TrainerId)); err != nil {
		return nil, err
	}
	if err := binary.Write(data, binary.BigEndian, int32(t.ClientId)); err != nil {
		return nil, err
	}
	if err := binary.Write(data, binary.BigEndian, t.PaymentDate.Unix()); err != nil {
		return nil, err
	}
	amountStr := t.Amount.String()
	if len(amountStr) > 32 {
		return nil, fmt.Errorf("amount too long")
	}
	amountBytes := make([]byte, 32)
	copy(amountBytes, amountStr)
	if _, err := data.Write(amountBytes); err != nil {
		return nil, err
	}
	if err := binary.Write(&buf, binary.BigEndian, int32(data.Len())); err != nil {
		return nil, err
	}
	if _, err := buf.Write(data.Bytes()); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
