package blockchain

import (
	"crypto/hmac"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/eightuponatime/centralized-blockchain/internal/models"
	"github.com/shopspring/decimal"
)

type Blockchain struct {
	Filename  string
	SecretKey []byte
}

func NewBlockchain(filename, secretKey string) *Blockchain {
	return &Blockchain{
		Filename:  filename,
		SecretKey: []byte(secretKey),
	}
}

func (bc *Blockchain) CreateGenesisBlock() (*models.Block, error) {
	f, err := os.Open(bc.Filename)
	if err == nil {
		info, err := f.Stat()
		if err != nil {
			f.Close()
			return nil, fmt.Errorf("failed to stat file: %v", err)
		}
		if info.Size() > 0 {
			f.Close()
			return nil, fmt.Errorf("genesis block already exists")
		}
		f.Close()
	}

	transaction := &models.Transaction{
		ClientId:    0,
		TrainerId:   0,
		PaymentDate: time.Now(),
		Amount:      decimal.Zero,
	}

	txData, err := transaction.SerializeTransaction()
	if err != nil {
		return nil, fmt.Errorf("failed to serialize genesis transaction: %v", err)
	}

	prevHash := make([]byte, 32)
	h := sha256.New()
	h.Write(txData)
	h.Write(prevHash)

	block := &models.Block{
		Index:       1,
		Hash:        h.Sum(nil),
		Transaction: transaction,
		PrevHash:    prevHash,
		Timestamp:   time.Now(),
	}

	block.HMAC, err = bc.CalculateHMAC(block)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate HMAC: %v", err)
	}

	_, err = bc.AddBlockToFile(block)
	if err != nil {
		return nil, fmt.Errorf("failed to save genesis block: %v", err)
	}

	return block, nil
}

func (bc *Blockchain) NewBlock(transaction *models.Transaction) (*models.Block, error) {
	txData, err := transaction.SerializeTransaction()
	if err != nil {
		return nil, fmt.Errorf("failed to serialize transaction: %v", err)
	}

	prevIndex, prevHash, err := bc.GetLastBlockInfo()
	if err != nil {
		return nil, fmt.Errorf("failed to get last block info: %v", err)
	}
	if prevIndex == 0 {
		return nil, fmt.Errorf("genesis block not found, create if first")
	}

	h := sha256.New()
	h.Write(txData)
	h.Write(prevHash)

	block := &models.Block{
		Index:       prevIndex + 1,
		Hash:        h.Sum(nil),
		Transaction: transaction,
		PrevHash:    prevHash,
		Timestamp:   time.Now(),
	}

	block.HMAC, err = bc.CalculateHMAC(block)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate HMAC: %v", err)
	}

	return block, nil
}

func (bc *Blockchain) AddBlockToFile(block *models.Block) (int64, error) {
	f, err := os.OpenFile(bc.Filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return 0, fmt.Errorf("failed to open file: %v", err)
	}
	defer f.Close()
	offset, err := f.Seek(0, io.SeekEnd)
	if err != nil {
		return 0, fmt.Errorf("failed to seek: %v", err)
	}
	data, err := SerializeBlock(block)
	if err != nil {
		return 0, fmt.Errorf("failed to serialize block: %v", err)
	}
	if _, err := f.Write(data); err != nil {
		return 0, fmt.Errorf("failed to write block: %v", err)
	}
	return offset, nil
}

func (bc *Blockchain) GetLastBlockInfo() (int, []byte, error) {
	f, err := os.Open(bc.Filename)
	if os.IsNotExist(err) {
		return 0, nil, fmt.Errorf("blockchain file doesn't exist")
	}
	if err != nil {
		return 0, nil, fmt.Errorf("failed to open file: %v", err)
	}
	defer f.Close()

	var lastBlock *models.Block

	for {
		// _, err := f.Seek(0, io.SeekCurrent)
		// if err != nil {
		// 	return 0, nil, fmt.Errorf("failed to seek: %v", err)
		// }
		block, err := deserializeBlock(f)
		if err == io.EOF {
			break
		}
		if err != nil {
			return 0, nil, fmt.Errorf("failed to deserialize block: %v", err)
		}
		lastBlock = block
	}

	if lastBlock == nil {
		return 0, nil, fmt.Errorf("no blocks found in blockchain")
	}

	return lastBlock.Index, lastBlock.Hash, nil
}

func (bc *Blockchain) CalculateHMAC(block *models.Block) ([]byte, error) {
	data, err := SerializeWithoutHMAC(block)
	if err != nil {
		return nil, err
	}
	h := hmac.New(sha256.New, bc.SecretKey)
	h.Write(data)

	return h.Sum(nil), nil
}
