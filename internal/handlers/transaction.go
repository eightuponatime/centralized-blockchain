package handlers

import (
	"fmt"
	"net/http"
	"time"

	"github.com/eightuponatime/centralized-blockchain/internal/blockchain"
	"github.com/eightuponatime/centralized-blockchain/internal/models"
	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
)

type TransactionRequest struct {
	ClientId    int     `json:"clientId" binding:"required,gt=0"`
	TrainerId   int     `json:"trainerId" binding:"required,gt=0"`
	Amount      float64 `json:"amount" binding:"required,gt=0"`
	PaymentDate string  `json:"paymentDate" binding:"required"`
}

type TransactionHandler struct {
	bc *blockchain.Blockchain
}

func NewTransactionHandler(bc *blockchain.Blockchain) *TransactionHandler {
	return &TransactionHandler{ bc: bc }
}

func (h *TransactionHandler) CreateGenesis(c *gin.Context) {
	block, err := h.bc.CreateGenesisBlock()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"block": block,
	})
}

func (h *TransactionHandler) CreateTransaction(c *gin.Context) {
	var req TransactionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	paymentDate, err := time.Parse(time.RFC3339, req.PaymentDate)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid paymentDate format use RFC3339"})
		return
	}

	transaction := &models.Transaction{
		ClientId:    req.ClientId,
		TrainerId:   req.TrainerId,
		PaymentDate: paymentDate,
		Amount:      decimal.NewFromFloat(req.Amount),
	}

	block, err := h.bc.NewBlock(transaction)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	_, err = h.bc.AddBlockToFile(block)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("failed to save block: %v", err)})
		return	
	}
	
	c.JSON(http.StatusCreated, gin.H {"block": block,})
}
