package models

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"time"

	"github.com/mattn/go-colorable"
	"github.com/rs/zerolog"
	"github.com/shopspring/decimal"
)

const (
	MaxTransactionPayloadSize = 1 * 1024 * 1024 // 1MB
	MaxAmountDataSize         = 100 * 1024      // 100KB
)

// Represents a blockchain transaction
type Transaction struct {
	TrainerId   int             `json:"trainerId"`
	ClientId    int             `json:"clientId"`
	PaymentDate time.Time       `json:"paymentDate"`
	Amount      decimal.Decimal `json:"amount"`
}

// Serializes the transaction to a byte slice
func (t *Transaction) Serialize() ([]byte, error) {
	var outerBuf, innerDataBuf bytes.Buffer

	if err := binary.Write(&innerDataBuf, binary.BigEndian, int32(t.TrainerId)); err != nil {
		return nil, fmt.Errorf("failed to write trainerId: %w", err)
	}
	if err := binary.Write(&innerDataBuf, binary.BigEndian, int32(t.ClientId)); err != nil {
		return nil, fmt.Errorf("failed to write clientId: %w", err)
	}
	if err := binary.Write(&innerDataBuf, binary.BigEndian, t.PaymentDate.Unix()); err != nil {
		return nil, fmt.Errorf("failed to write paymentDate: %w", err)
	}

	amountData, err := t.Amount.MarshalBinary()
	if err != nil {
		return nil, fmt.Errorf("failed to marshal amount: %w", err)
	}
	if err := binary.Write(&innerDataBuf, binary.BigEndian, int32(len(amountData))); err != nil {
		return nil, fmt.Errorf("failed to write amount length: %w", err)
	}
	if _, err := innerDataBuf.Write(amountData); err != nil {
		return nil, fmt.Errorf("failed to write amount data: %w", err)
	}

	payloadBytes := innerDataBuf.Bytes()
	if err := binary.Write(&outerBuf, binary.BigEndian, int32(len(payloadBytes))); err != nil {
		return nil, fmt.Errorf("failed to write payload length: %w", err)
	}
	if _, err := outerBuf.Write(payloadBytes); err != nil {
		return nil, fmt.Errorf("failed to write payload data: %w", err)
	}

	return outerBuf.Bytes(), nil
}

// Deserializes a transaction from a reader
func DeserializeTransaction(reader io.Reader) (*Transaction, error) {
	logger := zerolog.New(zerolog.ConsoleWriter{Out: colorable.NewColorableStdout()}).With().Timestamp().Logger()
	var totalPayloadLength int32
	if err := binary.Read(reader, binary.BigEndian, &totalPayloadLength); err != nil {
		return nil, fmt.Errorf("failed to read payload length: %w", err)
	}
	if totalPayloadLength <= 0 || totalPayloadLength > MaxTransactionPayloadSize {
		return nil, fmt.Errorf("invalid payload length: %d", totalPayloadLength)
	}

	payloadData := make([]byte, totalPayloadLength)
	if _, err := io.ReadFull(reader, payloadData); err != nil {
		return nil, fmt.Errorf("failed to read payload data: %w", err)
	}

	buf := bytes.NewReader(payloadData)
	var tx Transaction
	var trainerIdRead, clientIdRead int32
	var timestampRead int64

	if err := binary.Read(buf, binary.BigEndian, &trainerIdRead); err != nil {
		return nil, fmt.Errorf("failed to read trainerId: %w", err)
	}
	tx.TrainerId = int(trainerIdRead)

	if err := binary.Read(buf, binary.BigEndian, &clientIdRead); err != nil {
		return nil, fmt.Errorf("failed to read clientId: %w", err)
	}
	tx.ClientId = int(clientIdRead)

	if err := binary.Read(buf, binary.BigEndian, &timestampRead); err != nil {
		return nil, fmt.Errorf("failed to read paymentDate: %w", err)
	}
	tx.PaymentDate = time.Unix(timestampRead, 0)

	var amountLen int32
	if err := binary.Read(buf, binary.BigEndian, &amountLen); err != nil {
		return nil, fmt.Errorf("failed to read amount length: %w", err)
	}
	if amountLen < 0 || amountLen > MaxAmountDataSize {
		return nil, fmt.Errorf("invalid amount length: %d", amountLen)
	}

	if amountLen > 0 {
		amountData := make([]byte, amountLen)
		if _, err := io.ReadFull(buf, amountData); err != nil {
			return nil, fmt.Errorf("failed to read amount data: %w", err)
		}
		if err := tx.Amount.UnmarshalBinary(amountData); err != nil {
			return nil, fmt.Errorf("failed to unmarshal amount: %w", err)
		}
	} else {
		tx.Amount = decimal.Zero
	}

	if buf.Len() > 0 {
		logger.Warn().Int("remaining_bytes", buf.Len()).Msg("Unexpected bytes in transaction payload")
	}

	return &tx, nil
}
