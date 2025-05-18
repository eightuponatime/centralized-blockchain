package blockchain

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/gob"
	"errors"
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	"github.com/eightuponatime/centralized-blockchain/internal/models"
	"github.com/mattn/go-colorable"
	"github.com/rs/zerolog"
)

// Blockchain manages a blockchain stored in a file with an index.
type Blockchain struct {
	Filename      string
	SecretKey     []byte
	Index         *models.Index
	IndexFile     string
	lastBlockInfo struct {
		Index  int
		Hash   []byte
		Offset int64
	}
	mu     sync.Mutex
	logger zerolog.Logger
}

// NewBlockchain initializes a new blockchain with the given filename and secret key.
func NewBlockchain(filename, secretKey string) *Blockchain {
	logger := zerolog.New(zerolog.ConsoleWriter{Out: colorable.NewColorableStdout()}).With().Timestamp().Logger()
	indexFile := "/data/index.dat"
	//indexFile := "cmd/index.dat"
	index, err := loadIndex(indexFile)
	if err != nil {
		logger.Error().Err(err).Str("file", indexFile).Msg("Failed to load index")
	}
	bc := &Blockchain{
		Filename:  filename,
		SecretKey: []byte(secretKey),
		Index:     index,
		IndexFile: indexFile,
		logger:    logger,
	}
	if err := bc.loadLastBlockInfo(); err != nil {
		logger.Error().Err(err).Msg("Failed to load last block info")
	}
	logger.Info().Str("file", filename).Msg("Initialized blockchain")
	return bc
}

// loadIndex loads or creates the index from the index file.
func loadIndex(indexFile string) (*models.Index, error) {
	logger := zerolog.New(zerolog.ConsoleWriter{Out: colorable.NewColorableStdout()}).With().Timestamp().Logger()
	f, err := os.Open(indexFile)
	if os.IsNotExist(err) {
		logger.Info().Str("file", indexFile).Msg("Creating new index")
		return models.NewIndex(), nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to open index file %s: %w", indexFile, err)
	}
	defer f.Close()

	var index models.Index
	if err := gob.NewDecoder(f).Decode(&index); err != nil {
		logger.Error().Err(err).Str("file", indexFile).Msg("Failed to decode index")
		return models.NewIndex(), nil
	}

	if index.ByClientId == nil {
		index.ByClientId = make(map[int][]models.BlockIndex)
	}
	if index.ByClientTrainerId == nil {
		index.ByClientTrainerId = make(map[string][]models.BlockIndex)
	}
	logger.Info().Str("file", indexFile).Msg("Loaded index")
	return &index, nil
}

// saveIndex saves the index to the index file.
func saveIndex(index *models.Index, indexFile string) error {
	logger := zerolog.New(zerolog.ConsoleWriter{Out: colorable.NewColorableStdout()}).With().Timestamp().Logger()
	f, err := os.Create(indexFile)
	if err != nil {
		return fmt.Errorf("failed to create index file %s: %w", indexFile, err)
	}
	defer f.Close()

	if err := gob.NewEncoder(f).Encode(index); err != nil {
		return fmt.Errorf("failed to encode index to %s: %w", indexFile, err)
	}
	logger.Info().Str("file", indexFile).Msg("Saved index")
	return nil
}

// calculateHMAC computes the HMAC for a block.
func (bc *Blockchain) calculateHMAC(block *models.Block) ([]byte, error) {
	data, err := block.SerializeWithoutHMAC()
	if err != nil {
		return nil, fmt.Errorf("failed to serialize block %d for HMAC: %w", block.Index, err)
	}
	h := hmac.New(sha256.New, bc.SecretKey)
	h.Write(data)
	return h.Sum(nil), nil
}

// CreateGenesisBlock creates the genesis block with the given transaction.
func (bc *Blockchain) CreateGenesisBlock(transaction *models.Transaction) (*models.Block, error) {
	bc.mu.Lock()
	defer bc.mu.Unlock()
	bc.logger.Info().Interface("transaction", transaction).Msg("Creating genesis block")

	if bc.lastBlockInfo.Index != 0 {
		return nil, fmt.Errorf("genesis block exists, last block index: %d", bc.lastBlockInfo.Index)
	}

	h := sha256.New()
	h.Write([]byte("genesis_prev_hash_seed"))
	prevHash := h.Sum(nil)

	txData, err := transaction.Serialize()
	if err != nil {
		return nil, fmt.Errorf("failed to serialize transaction: %w", err)
	}

	hashInput := bytes.NewBuffer(txData)
	hashInput.Write(prevHash)
	blockHash := sha256.Sum256(hashInput.Bytes())

	block := &models.Block{
		Index:       1,
		Hash:        blockHash[:],
		Transaction: transaction,
		PrevHash:    prevHash,
		Timestamp:   time.Now(),
	}

	if block.HMAC, err = bc.calculateHMAC(block); err != nil {
		return nil, fmt.Errorf("failed to calculate HMAC: %w", err)
	}

	offset, err := bc.AddBlockToFile(block)
	if err != nil {
		return nil, fmt.Errorf("failed to add genesis block: %w", err)
	}

	bc.lastBlockInfo = struct {
		Index  int
		Hash   []byte
		Offset int64
	}{block.Index, block.Hash, offset}
	bc.logger.Info().
		Int("index", block.Index).
		Int64("offset", offset).
		Str("hash", fmt.Sprintf("%x", block.Hash)).
		Msg("Created genesis block")
	return block, nil
}

// AddBlockToFile appends a block to the blockchain file and updates the index.
func (bc *Blockchain) AddBlockToFile(block *models.Block) (int64, error) {
	f, err := os.OpenFile(bc.Filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return 0, fmt.Errorf("failed to open file %s: %w", bc.Filename, err)
	}
	defer f.Close()

	offset, err := f.Seek(0, io.SeekEnd)
	if err != nil {
		return 0, fmt.Errorf("failed to seek to end of file %s: %w", bc.Filename, err)
	}

	data, err := models.SerializeBlock(block)
	if err != nil {
		return 0, fmt.Errorf("failed to serialize block %d: %w", block.Index, err)
	}

	if _, err := f.Write(data); err != nil {
		return 0, fmt.Errorf("failed to write block %d to %s: %w", block.Index, bc.Filename, err)
	}

	bc.Index.Add(block, offset)
	if err := saveIndex(bc.Index, bc.IndexFile); err != nil {
		return 0, fmt.Errorf("failed to save index for block %d: %w", block.Index, err)
	}
	bc.logger.Info().Int("index", block.Index).Int64("offset", offset).Msg("Added block")
	return offset, nil
}

// NewBlock creates a new block with the given transaction.
func (bc *Blockchain) NewBlock(transaction *models.Transaction) (*models.Block, error) {
	bc.mu.Lock()
	defer bc.mu.Unlock()
	bc.logger.Info().Interface("transaction", transaction).Msg("Creating block")

	prevIndex, prevHash, err := bc.getLastBlockInfoInternal()
	if err != nil {
		return nil, fmt.Errorf("failed to get last block info: %w", err)
	}
	if prevIndex == 0 {
		return nil, fmt.Errorf("genesis block not found")
	}

	txData, err := transaction.Serialize()
	if err != nil {
		return nil, fmt.Errorf("failed to serialize transaction: %w", err)
	}

	hashInput := bytes.NewBuffer(txData)
	hashInput.Write(prevHash)
	blockHash := sha256.Sum256(hashInput.Bytes())

	block := &models.Block{
		Index:       prevIndex + 1,
		Hash:        blockHash[:],
		Transaction: transaction,
		PrevHash:    prevHash,
		Timestamp:   time.Now(),
	}

	if block.HMAC, err = bc.calculateHMAC(block); err != nil {
		return nil, fmt.Errorf("failed to calculate HMAC: %w", err)
	}

	offset, err := bc.AddBlockToFile(block)
	if err != nil {
		return nil, fmt.Errorf("failed to add block %d: %w", block.Index, err)
	}

	bc.lastBlockInfo = struct {
		Index  int
		Hash   []byte
		Offset int64
	}{block.Index, block.Hash, offset}
	bc.logger.Info().
		Int("index", block.Index).
		Int64("offset", offset).
		Str("hash", fmt.Sprintf("%x", block.Hash)).
		Msg("Created block")
	return block, nil
}

// GetLastBlockInfo returns the index and hash of the last block.
func (bc *Blockchain) GetLastBlockInfo() (int, []byte, error) {
	bc.mu.Lock()
	defer bc.mu.Unlock()
	return bc.getLastBlockInfoInternal()
}

// getLastBlockInfoInternal retrieves the last block info without locking (assumes mutex is held).
func (bc *Blockchain) getLastBlockInfoInternal() (int, []byte, error) {
	if bc.lastBlockInfo.Index == 0 {
		return 0, nil, nil
	}
	return bc.lastBlockInfo.Index, bc.lastBlockInfo.Hash, nil
}

// loadLastBlockInfo loads the last block info from the blockchain file.
func (bc *Blockchain) loadLastBlockInfo() error {
	f, err := os.Open(bc.Filename)
	if os.IsNotExist(err) {
		bc.logger.Info().Str("file", bc.Filename).Msg("File does not exist")
		return nil
	}
	if err != nil {
		return fmt.Errorf("failed to open %s: %w", bc.Filename, err)
	}
	defer f.Close()

	var lastBlock *models.Block
	var lastOffset int64

	for {
		offset, err := f.Seek(0, io.SeekCurrent)
		if err != nil {
			return fmt.Errorf("failed to seek at offset %d: %w", offset, err)
		}
		block, err := models.DeserializeBlock(f)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return fmt.Errorf("failed to deserialize block at offset %d: %w", offset, err)
		}
		lastBlock = block
		lastOffset = offset
	}

	if lastBlock != nil {
		bc.lastBlockInfo = struct {
			Index  int
			Hash   []byte
			Offset int64
		}{lastBlock.Index, lastBlock.Hash, lastOffset}
		bc.logger.Info().
			Int("index", lastBlock.Index).
			Int64("offset", lastOffset).
			Msg("Loaded last block")
	} else {
		bc.logger.Info().Str("file", bc.Filename).Msg("No blocks found")
	}
	return nil
}

// FindBlocksByClientId retrieves blocks by client ID using the index.
func (bc *Blockchain) FindBlocksByClientId(clientId int) ([]*models.Block, error) {
	blockIndices, ok := bc.Index.ByClientId[clientId]
	if !ok || len(blockIndices) == 0 {
		return []*models.Block{}, nil
	}

	f, err := os.Open(bc.Filename)
	if err != nil {
		return nil, fmt.Errorf("failed to open %s: %w", bc.Filename, err)
	}
	defer f.Close()

	var blocks []*models.Block
	for _, idx := range blockIndices {
		if _, err := f.Seek(idx.Offset, io.SeekStart); err != nil {
			return nil, fmt.Errorf("failed to seek to offset %d: %w", idx.Offset, err)
		}
		block, err := models.DeserializeBlock(f)
		if err != nil {
			return nil, fmt.Errorf("failed to deserialize block at offset %d: %w", idx.Offset, err)
		}
		if !bytes.Equal(block.Hash, idx.BlockHash) {
			return nil, fmt.Errorf("hash mismatch at offset %d", idx.Offset)
		}
		if valid, err := block.VerifyHMAC(bc.SecretKey); err != nil || !valid {
			return nil, fmt.Errorf("invalid HMAC for block %d at offset %d", block.Index, idx.Offset)
		}
		blocks = append(blocks, block)
	}
	bc.logger.Info().Int("count", len(blocks)).Int("client_id", clientId).Msg("Found blocks")
	return blocks, nil
}

// FindBlocksByClientTrainerId retrieves blocks by client and trainer IDs using the index.
func (bc *Blockchain) FindBlocksByClientTrainerId(clientId, trainerId int) ([]*models.Block, error) {
	key := fmt.Sprintf("%d:%d", clientId, trainerId)
	blockIndices, ok := bc.Index.ByClientTrainerId[key]
	if !ok || len(blockIndices) == 0 {
		return []*models.Block{}, nil
	}

	f, err := os.Open(bc.Filename)
	if err != nil {
		return nil, fmt.Errorf("failed to open %s: %w", bc.Filename, err)
	}
	defer f.Close()

	var blocks []*models.Block
	for _, idx := range blockIndices {
		if _, err := f.Seek(idx.Offset, io.SeekStart); err != nil {
			return nil, fmt.Errorf("failed to seek to offset %d: %w", idx.Offset, err)
		}
		block, err := models.DeserializeBlock(f)
		if err != nil {
			return nil, fmt.Errorf("failed to deserialize block at offset %d: %w", idx.Offset, err)
		}
		if !bytes.Equal(block.Hash, idx.BlockHash) {
			return nil, fmt.Errorf("hash mismatch at offset %d", idx.Offset)
		}
		if valid, err := block.VerifyHMAC(bc.SecretKey); err != nil || !valid {
			return nil, fmt.Errorf("invalid HMAC for block %d at offset %d", block.Index, idx.Offset)
		}
		blocks = append(blocks, block)
	}
	bc.logger.Info().
		Int("count", len(blocks)).
		Int("client_id", clientId).
		Int("trainer_id", trainerId).
		Msg("Found blocks")
	return blocks, nil
}

// FindBlocksByClientIdWithoutIndex retrieves blocks by client ID by scanning the entire file.
func (bc *Blockchain) FindBlocksByClientIdWithoutIndex(clientId int) ([]*models.Block, error) {
	f, err := os.Open(bc.Filename)
	if os.IsNotExist(err) {
		return []*models.Block{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to open %s: %w", bc.Filename, err)
	}
	defer f.Close()

	var blocks []*models.Block
	for {
		offset, err := f.Seek(0, io.SeekCurrent)
		if err != nil {
			return nil, fmt.Errorf("failed to seek at offset %d: %w", offset, err)
		}
		block, err := models.DeserializeBlock(f)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to deserialize block at offset %d: %w", offset, err)
		}
		if valid, err := block.VerifyHMAC(bc.SecretKey); err != nil || !valid {
			return nil, fmt.Errorf("invalid HMAC for block %d at offset %d", block.Index, offset)
		}
		if block.Transaction != nil && block.Transaction.ClientId == clientId {
			blocks = append(blocks, block)
		}
	}
	bc.logger.Info().Int("count", len(blocks)).Int("client_id", clientId).Msg("Found blocks without index")
	return blocks, nil
}

// FindBlocksByClientTrainerIdWithoutIndex retrieves blocks by client and trainer IDs by scanning the entire file.
func (bc *Blockchain) FindBlocksByClientTrainerIdWithoutIndex(clientId, trainerId int) ([]*models.Block, error) {
	f, err := os.Open(bc.Filename)
	if os.IsNotExist(err) {
		return []*models.Block{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to open %s: %w", bc.Filename, err)
	}
	defer f.Close()

	var blocks []*models.Block
	for {
		offset, err := f.Seek(0, io.SeekCurrent)
		if err != nil {
			return nil, fmt.Errorf("failed to seek at offset %d: %w", offset, err)
		}
		block, err := models.DeserializeBlock(f)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to deserialize block at offset %d: %w", offset, err)
		}
		if valid, err := block.VerifyHMAC(bc.SecretKey); err != nil || !valid {
			return nil, fmt.Errorf("invalid HMAC for block %d at offset %d", block.Index, offset)
		}
		if block.Transaction != nil && block.Transaction.ClientId == clientId && block.Transaction.TrainerId == trainerId {
			blocks = append(blocks, block)
		}
	}
	bc.logger.Info().
		Int("count", len(blocks)).
		Int("client_id", clientId).
		Int("trainer_id", trainerId).
		Msg("Found blocks without index")
	return blocks, nil
}

// GetAllBlocks retrieves all blocks from the blockchain file.
func (bc *Blockchain) GetAllBlocks() ([]*models.Block, error) {
	f, err := os.Open(bc.Filename)
	if os.IsNotExist(err) {
		return []*models.Block{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to open %s: %w", bc.Filename, err)
	}
	defer f.Close()

	var blocks []*models.Block
	for {
		offset, err := f.Seek(0, io.SeekCurrent)
		if err != nil {
			return nil, fmt.Errorf("failed to seek at offset %d: %w", offset, err)
		}
		block, err := models.DeserializeBlock(f)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to deserialize block at offset %d: %w", offset, err)
		}
		if valid, err := block.VerifyHMAC(bc.SecretKey); err != nil || !valid {
			return nil, fmt.Errorf("invalid HMAC for block %d at offset %d", block.Index, offset)
		}
		blocks = append(blocks, block)
	}
	bc.logger.Info().Int("count", len(blocks)).Msg("Retrieved blocks")
	return blocks, nil
}

// VerifyChain verifies the integrity of the blockchain.
func (bc *Blockchain) VerifyChain() (bool, error) {
	f, err := os.Open(bc.Filename)
	if os.IsNotExist(err) {
		return true, nil
	}
	if err != nil {
		return false, fmt.Errorf("failed to open %s: %w", bc.Filename, err)
	}
	defer f.Close()

	var prevBlock *models.Block
	expectedIndex := 1

	for {
		offset, err := f.Seek(0, io.SeekCurrent)
		if err != nil {
			return false, fmt.Errorf("failed to seek at offset %d: %w", offset, err)
		}
		block, err := models.DeserializeBlock(f)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return false, fmt.Errorf("failed to deserialize block at offset %d: %w", offset, err)
		}
		if block.Index != expectedIndex {
			return false, fmt.Errorf("invalid index %d at offset %d, expected %d", block.Index, offset, expectedIndex)
		}
		if valid, err := block.VerifyHMAC(bc.SecretKey); err != nil || !valid {
			return false, fmt.Errorf("invalid HMAC for block %d at offset %d", block.Index, offset)
		}
		if prevBlock != nil && !bytes.Equal(block.PrevHash, prevBlock.Hash) {
			return false, fmt.Errorf("prevHash mismatch at index %d, offset %d", block.Index, offset)
		}
		prevBlock = block
		expectedIndex++
	}
	bc.logger.Info().Msg("Verification passed")
	return true, nil
}
