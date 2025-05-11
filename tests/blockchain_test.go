package blockchain

import (
	"crypto/hmac"
	"crypto/sha256"
	"os"
	"testing"
	"time"

	"github.com/eightuponatime/centralized-blockchain/internal/blockchain"
	"github.com/eightuponatime/centralized-blockchain/internal/models"
	"github.com/shopspring/decimal"
)

// TestBlock_VerifyHMAC tests the HMAC verification of a block.
func TestBlock_VerifyHMAC(t *testing.T) {
	secretKey := []byte("test-key")
	block := &models.Block{
		Index:     1,
		Hash:      make([]byte, 32),
		PrevHash:  make([]byte, 32),
		Timestamp: time.Now(),
		HMAC:      make([]byte, 32),
		Transaction: &models.Transaction{
			ClientId:    1,
			TrainerId:   2,
			Amount:      decimal.NewFromInt(100),
			PaymentDate: time.Now(),
		},
	}
	data, err := block.SerializeWithoutHMAC()
	if err != nil {
		t.Fatalf("Failed to serialize block: %v", err)
	}
	h := hmac.New(sha256.New, secretKey)
	h.Write(data)
	block.HMAC = h.Sum(nil)
	valid, err := block.VerifyHMAC(secretKey)
	if err != nil || !valid {
		t.Fatalf("Expected valid HMAC, got error=%v, valid=%v", err, valid)
	}
}

// TestBlockchain_AddBlockToFile tests adding a block to the blockchain file.
func TestBlockchain_AddBlockToFile(t *testing.T) {
	filename := "test_blockchain.dat"
	indexFile := "index.dat"
	defer os.Remove(filename)
	defer os.Remove(indexFile)

	bc := blockchain.NewBlockchain(filename, "test-key")
	tx := &models.Transaction{
		ClientId:    1,
		TrainerId:   2,
		Amount:      decimal.NewFromInt(100),
		PaymentDate: time.Now(),
	}
	block, err := bc.CreateGenesisBlock(tx)
	if err != nil {
		t.Fatalf("Failed to create genesis block: %v", err)
	}

	offset, err := bc.AddBlockToFile(block)
	if err != nil {
		t.Fatalf("Failed to add block to file: %v", err)
	}
	if offset < 0 {
		t.Fatalf("Expected positive offset, got %d", offset)
	}
}

// TestBlockchain_FindBlocksByClientId tests finding blocks by client ID.
func TestBlockchain_FindBlocksByClientId(t *testing.T) {
	filename := "test_blockchain.dat"
	indexFile := "index.dat"
	defer os.Remove(filename)
	defer os.Remove(indexFile)

	bc := blockchain.NewBlockchain(filename, "test-key")
	tx1 := &models.Transaction{
		ClientId:    1,
		TrainerId:   2,
		Amount:      decimal.NewFromInt(100),
		PaymentDate: time.Now(),
	}
	tx2 := &models.Transaction{
		ClientId:    1,
		TrainerId:   3,
		Amount:      decimal.NewFromInt(200),
		PaymentDate: time.Now(),
	}
	_, err := bc.CreateGenesisBlock(tx1)
	if err != nil {
		t.Fatalf("Failed to create genesis block: %v", err)
	}
	_, err = bc.NewBlock(tx2)
	if err != nil {
		t.Fatalf("Failed to add new block: %v", err)
	}

	blocks, err := bc.FindBlocksByClientId(1)
	if err != nil {
		t.Fatalf("Failed to find blocks: %v", err)
	}
	if len(blocks) != 2 {
		t.Fatalf("Expected 2 blocks, got %d", len(blocks))
	}
}

// TestBlockchain_GetAllBlocks tests retrieving all blocks.
func TestBlockchain_GetAllBlocks(t *testing.T) {
	filename := "test_blockchain.dat"
	indexFile := "index.dat"
	defer os.Remove(filename)
	defer os.Remove(indexFile)

	bc := blockchain.NewBlockchain(filename, "test-key")
	tx1 := &models.Transaction{
		ClientId:    1,
		TrainerId:   2,
		Amount:      decimal.NewFromInt(100),
		PaymentDate: time.Now(),
	}
	tx2 := &models.Transaction{
		ClientId:    2,
		TrainerId:   3,
		Amount:      decimal.NewFromInt(200),
		PaymentDate: time.Now(),
	}
	_, err := bc.CreateGenesisBlock(tx1)
	if err != nil {
		t.Fatalf("Failed to create genesis block: %v", err)
	}
	_, err = bc.NewBlock(tx2)
	if err != nil {
		t.Fatalf("Failed to add new block: %v", err)
	}

	blocks, err := bc.GetAllBlocks()
	if err != nil {
		t.Fatalf("Failed to get all blocks: %v", err)
	}
	if len(blocks) != 2 {
		t.Fatalf("Expected 2 blocks, got %d", len(blocks))
	}
}

// TestBlockchain_VerifyChain tests the blockchain integrity verification.
func TestBlockchain_VerifyChain(t *testing.T) {
	filename := "test_blockchain.dat"
	indexFile := "index.dat"
	defer os.Remove(filename)
	defer os.Remove(indexFile)

	bc := blockchain.NewBlockchain(filename, "test-key")
	tx1 := &models.Transaction{
		ClientId:    1,
		TrainerId:   2,
		Amount:      decimal.NewFromInt(100),
		PaymentDate: time.Now(),
	}
	tx2 := &models.Transaction{
		ClientId:    2,
		TrainerId:   3,
		Amount:      decimal.NewFromInt(200),
		PaymentDate: time.Now(),
	}
	_, err := bc.CreateGenesisBlock(tx1)
	if err != nil {
		t.Fatalf("Failed to create genesis block: %v", err)
	}
	_, err = bc.NewBlock(tx2)
	if err != nil {
		t.Fatalf("Failed to add new block: %v", err)
	}

	valid, err := bc.VerifyChain()
	if err != nil || !valid {
		t.Fatalf("Chain verification failed: error=%v, valid=%v", err, valid)
	}
}