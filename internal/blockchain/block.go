package blockchain

import (
	"crypto/sha256"

	"github.com/eightuponatime/centralized-blockchain/internal/models"
)

type Block struct {
	Hash     []byte
	Data     models.Payment
	PrevHash []byte
}

func NewBlock(date *models.Payment, prevHash []byte) *Block {
	hash := sha256.New()
	_ = hash
}
