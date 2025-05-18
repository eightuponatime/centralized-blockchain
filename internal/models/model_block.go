package models

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"io"
	"time"

	"github.com/mattn/go-colorable"
	"github.com/rs/zerolog"
)

const (
	MaxBlockDataWithoutHMACSize = 10 * 1024 * 1024 // 10MB
	Sha256HashSize              = 32               // SHA256 hash size
)

// Block represents a block in the blockchain.
type Block struct {
	Index       int
	Hash        []byte
	Transaction *Transaction
	PrevHash    []byte
	Timestamp   time.Time
	HMAC        []byte
}

// BlockIndex represents an index entry for a block.
type BlockIndex struct {
	ClientId  int
	TrainerId int
	Offset    int64
	BlockHash []byte
	HMAC      []byte
}

// Index manages block indices for efficient lookup.
type Index struct {
	ByClientId        map[int][]BlockIndex
	ByClientTrainerId map[string][]BlockIndex 
}

// NewIndex creates a new index.
func NewIndex() *Index {
	return &Index{
		ByClientId:        make(map[int][]BlockIndex),
		ByClientTrainerId: make(map[string][]BlockIndex),
	}
}

// Add adds a block to the index.
func (idx *Index) Add(block *Block, offset int64) {
	logger := zerolog.New(zerolog.ConsoleWriter{Out: colorable.NewColorableStdout()}).With().Timestamp().Logger()
	if block == nil || block.Transaction == nil {
		logger.Error().Msg("Nil block or transaction")
		return
	}
	clientId := block.Transaction.ClientId
	trainerId := block.Transaction.TrainerId
	entry := BlockIndex{
		ClientId:  clientId,
		TrainerId: trainerId,
		Offset:    offset,
		BlockHash: block.Hash,
		HMAC:      block.HMAC,
	}
	idx.ByClientId[clientId] = append(idx.ByClientId[clientId], entry)
	key := fmt.Sprintf("%d:%d", clientId, trainerId)
	idx.ByClientTrainerId[key] = append(idx.ByClientTrainerId[key], entry)
}

// VerifyHMAC verifies the block's HMAC using the provided secret key.
func (b *Block) VerifyHMAC(secretKey []byte) (bool, error) {
	if b.HMAC == nil {
		return false, fmt.Errorf("HMAC is nil for block %d", b.Index)
	}
	expectedHMAC, err := b.CalculateHMAC(secretKey)
	if err != nil {
		return false, fmt.Errorf("failed to calculate HMAC: %w", err)
	}
	return hmac.Equal(b.HMAC, expectedHMAC), nil
}

// CalculateHMAC computes the HMAC for the block.
func (b *Block) CalculateHMAC(secretKey []byte) ([]byte, error) {
	data, err := b.SerializeWithoutHMAC()
	if err != nil {
		return nil, fmt.Errorf("failed to serialize block %d: %w", b.Index, err)
	}
	h := hmac.New(sha256.New, secretKey)
	h.Write(data)
	return h.Sum(nil), nil
}

// SerializeWithoutHMAC serializes the block without the HMAC field.
func (b *Block) SerializeWithoutHMAC() ([]byte, error) {
	var buf bytes.Buffer

	if err := binary.Write(&buf, binary.BigEndian, int32(b.Index)); err != nil {
		return nil, fmt.Errorf("failed to write index: %w", err)
	}
	if err := binary.Write(&buf, binary.BigEndian, b.Timestamp.UnixNano()); err != nil {
		return nil, fmt.Errorf("failed to write timestamp: %w", err)
	}
	if _, err := buf.Write(b.PrevHash); err != nil {
		return nil, fmt.Errorf("failed to write prevHash: %w", err)
	}
	if _, err := buf.Write(b.Hash); err != nil {
		return nil, fmt.Errorf("failed to write hash: %w", err)
	}
	if b.Transaction == nil {
		return nil, fmt.Errorf("transaction is nil for block %d", b.Index)
	}
	txData, err := b.Transaction.Serialize()
	if err != nil {
		return nil, fmt.Errorf("failed to serialize transaction: %w", err)
	}
	if _, err := buf.Write(txData); err != nil {
		return nil, fmt.Errorf("failed to write transaction: %w", err)
	}
	return buf.Bytes(), nil
}

// SerializeBlock serializes the entire block for storage.
func SerializeBlock(b *Block) ([]byte, error) {
	var buf bytes.Buffer

	data, err := b.SerializeWithoutHMAC()
	if err != nil {
		return nil, fmt.Errorf("failed to serialize block %d: %w", b.Index, err)
	}
	if err := binary.Write(&buf, binary.BigEndian, int32(len(data))); err != nil {
		return nil, fmt.Errorf("failed to write data length: %w", err)
	}
	if _, err := buf.Write(data); err != nil {
		return nil, fmt.Errorf("failed to write block data: %w", err)
	}
	if _, err := buf.Write(b.HMAC); err != nil {
		return nil, fmt.Errorf("failed to write HMAC: %w", err)
	}
	return buf.Bytes(), nil
}

// DeserializeBlock deserializes a block from a reader.
func DeserializeBlock(reader io.Reader) (*Block, error) {
	var length int32
	if err := binary.Read(reader, binary.BigEndian, &length); err != nil {
		return nil, fmt.Errorf("failed to read data length: %w", err)
	}
	if length <= 0 || length > MaxBlockDataWithoutHMACSize {
		return nil, fmt.Errorf("invalid data length: %d", length)
	}

	data := make([]byte, length)
	if _, err := io.ReadFull(reader, data); err != nil {
		return nil, fmt.Errorf("failed to read block data: %w", err)
	}

	buf := bytes.NewReader(data)
	var block Block
	var indexRead int32
	var timestampNanoRead int64

	if err := binary.Read(buf, binary.BigEndian, &indexRead); err != nil {
		return nil, fmt.Errorf("failed to read index: %w", err)
	}
	block.Index = int(indexRead)

	if err := binary.Read(buf, binary.BigEndian, &timestampNanoRead); err != nil {
		return nil, fmt.Errorf("failed to read timestamp: %w", err)
	}
	block.Timestamp = time.Unix(0, timestampNanoRead)

	block.PrevHash = make([]byte, Sha256HashSize)
	if _, err := io.ReadFull(buf, block.PrevHash); err != nil {
		return nil, fmt.Errorf("failed to read prevHash: %w", err)
	}

	block.Hash = make([]byte, Sha256HashSize)
	if _, err := io.ReadFull(buf, block.Hash); err != nil {
		return nil, fmt.Errorf("failed to read hash: %w", err)
	}

	tx, err := DeserializeTransaction(buf)
	if err != nil {
		return nil, fmt.Errorf("failed to deserialize transaction: %w", err)
	}
	block.Transaction = tx

	block.HMAC = make([]byte, Sha256HashSize)
	if _, err := io.ReadFull(reader, block.HMAC); err != nil {
		return nil, fmt.Errorf("failed to read HMAC: %w", err)
	}
	return &block, nil
}
