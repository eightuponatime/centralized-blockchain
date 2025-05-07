package main

import (
	"crypto/sha256"
	"fmt"
	"time"

	"github.com/shopspring/decimal"
)

type Payment struct {
	TrainerId   int             `json:"trainerId"`
	ClientId    int             `json:"clientId"`
	PaymentDate time.Time       `json:"paymentDate"`
	Amount      decimal.Decimal `json:"amount"`
}

type Block struct {
	Index       int
	Hash        []byte
	Transaction *Payment
	PrevHash    []byte
	Timestamp   time.Time
}

func createNewBlock(transaction *Payment, prevHash []byte, prevIndex int) *Block {
	h := sha256.New()

	transactionString := fmt.Sprintf("%d%d%s%s",
		transaction.TrainerId,
		transaction.ClientId,
		transaction.PaymentDate.Format(time.RFC3339),
		transaction.Amount.String(),
	)

	h.Write([]byte(transactionString))
	h.Write(prevHash)

	block := &Block{
		Index:       prevIndex + 1,
		Hash:        h.Sum(nil),
		Transaction: transaction,
		PrevHash:    prevHash,
		Timestamp:   time.Now(),
	}

	return block
}

func main() {
	hash := sha256.New()
	hash.Write([]byte("genesis"))
	previousHash := hash.Sum(nil)

	payment := initTransaction()
	block := createNewBlock(payment, previousHash, 0)

	fmt.Printf("Block Index: %d\n", block.Index)
	fmt.Printf("Block Hash: %x\n", block.Hash)
	fmt.Printf("Prev Hash: %x\n", block.PrevHash)
	fmt.Printf("Transaction: %+v\n", block.Transaction)
}

func initTransaction() *Payment {
	payment := &Payment{
		TrainerId:   1,
		ClientId:    2,
		PaymentDate: time.Now(),
		Amount:      decimal.NewFromFloat(125124.55),
	}
	return payment
}
