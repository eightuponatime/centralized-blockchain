package blockchain

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"time"

	"github.com/eightuponatime/centralized-blockchain/internal/models"
	"github.com/shopspring/decimal"
)

func SerializeWithoutHMAC(block *models.Block) ([]byte, error) {
	var buf bytes.Buffer
	if err := binary.Write(&buf, binary.BigEndian, int32(block.Index)); err != nil {
		return nil, err
	}
	if err := binary.Write(&buf, binary.BigEndian, block.Timestamp.Unix()); err != nil {
		return nil, err
	}
	if _, err := buf.Write(block.PrevHash); err != nil {
		return nil, err
	}
	if _, err := buf.Write(block.Hash); err != nil {
		return nil, err
	}

	txData, err := block.Transaction.SerializeTransaction()
	if err != nil {
		return nil, err
	}
	if _, err := buf.Write(txData); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func SerializeBlock(block *models.Block) ([]byte, error) {
	var buf bytes.Buffer

	data, err := SerializeWithoutHMAC(block)
	if err != nil {
		return nil, err
	}
	if err := binary.Write(&buf, binary.BigEndian, int32(len(data))); err != nil {
		return nil, err
	}
	if _, err := buf.Write(data); err != nil {
		return nil, err
	}
	if _, err := buf.Write(block.HMAC); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func deserializeBlock(reader io.Reader) (*models.Block, error) {
	var length int32
	if err := binary.Read(reader, binary.BigEndian, &length); err != nil {
		return nil, fmt.Errorf("failed to read block length: %v", err)
	}

	data := make([]byte, length)
	if _, err := io.ReadFull(reader, data); err != nil {
		return nil, fmt.Errorf("failed to read block data: %v", err)
	}
	buf := bytes.NewReader(data)
	var block models.Block
	var index int32
	var timestamp int64

	if err := binary.Read(buf, binary.BigEndian, &index); err != nil {
		return nil, fmt.Errorf("failed to read index: %v", err)
	}
	block.Index = int(index)

	if err := binary.Read(buf, binary.BigEndian, &timestamp); err != nil {
		return nil, fmt.Errorf("failed to read timestamp: %v", err)
	}
	block.Timestamp = time.Unix(timestamp, 0)

	block.PrevHash = make([]byte, 32)

	if _, err := buf.Read(block.PrevHash); err != nil {
		return nil, fmt.Errorf("failed to read prevHash: %v", err)
	}

	block.Hash = make([]byte, 32)
	if _, err := buf.Read(block.Hash); err != nil {
		return nil, fmt.Errorf("failed to read hash: %v", err)
	}

	tx, err := deserializeTransaction(buf)
	if err != nil {
		return nil, fmt.Errorf("failed to deserialize transaction: %v", err)
	}
	block.Transaction = tx

	block.HMAC = make([]byte, 32)
	if _, err := reader.Read(block.HMAC); err != nil {
		return nil, fmt.Errorf("failed to read HMAC: %v", err)
	}

	log.Printf("Deserialized block with Index=%d", block.Index)

	return &block, nil
}

func deserializeTransaction(reader io.Reader) (*models.Transaction, error) {
	var length int32
	if err := binary.Read(reader, binary.BigEndian, &length); err != nil {
		return nil, fmt.Errorf("failed to read transaction length: %v", err)
	}
	data := make([]byte, length)
	if _, err := io.ReadFull(reader, data); err != nil {
		return nil, fmt.Errorf("failed to read transaction data: %v", err)
	}
	buf := bytes.NewReader(data)
	var tx models.Transaction
	var trainerId, clientId int32
	var timestamp int64

	if err := binary.Read(buf, binary.BigEndian, &trainerId); err != nil {
		return nil, fmt.Errorf("failed to reader trainerId: %v", err)
	}
	tx.TrainerId = int(trainerId)

	if err := binary.Read(buf, binary.BigEndian, &clientId); err != nil {
		return nil, fmt.Errorf("failed to read clientId: %v", err)
	}
	tx.ClientId = int(clientId)

	if err := binary.Read(buf, binary.BigEndian, &timestamp); err != nil {
		return nil, fmt.Errorf("failed to read timestamp: %v", err)
	}
	tx.PaymentDate = time.Unix(timestamp, 0)

	amountBytes := make([]byte, 32)
	if _, err := buf.Read(amountBytes); err != nil {
		return nil, fmt.Errorf("failed to read amount: %v", err)
	}
	amountStr := string(bytes.TrimRight(amountBytes, "\x00"))
	amount, err := decimal.NewFromString(amountStr)
	if err != nil {
		return nil, fmt.Errorf("failed to parse amount: %v", err)
	}
	tx.Amount = amount

	return &tx, nil
}
