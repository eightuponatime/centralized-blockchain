package transaction

import (
	"github.com/eightuponatime/centralized-blockchain/internal/blockchain"
	"github.com/eightuponatime/centralized-blockchain/internal/models"
)

type TransactionHandler struct {
	bc *blockchain.Blockchain
}

// creates a new transaction handler
func NewTransactionHandler(bc *blockchain.Blockchain) *TransactionHandler {
	return &TransactionHandler{bc: bc}
}

// creates the genesis block with the given transaction
func (th *TransactionHandler) CreateGenesisBlock(transaction *models.Transaction) (*models.Block, error) {
	return th.bc.CreateGenesisBlock(transaction)
}

// creates a new block with the given transaction
func (th *TransactionHandler) NewTransaction(transaction *models.Transaction) (*models.Block, error) {
	return th.bc.NewBlock(transaction)
}
