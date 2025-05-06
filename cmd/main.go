package main

import (
	"time"

	"github.com/eightuponatime/centralized-blockchain/internal/blockchain"
	"github.com/eightuponatime/centralized-blockchain/internal/models"
	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
)

func main() {
	router := gin.Default()

	payment := models.Payment{
		ClientId:    1,
		TrainerId:   2,
		PaymentDate: time.Now(),
		Amount:      decimal.NewFromFloat(100.50),
	}

	block := blockchain.NewBlock(&payment, []byte{})
	_ = block

	router.Run("localhost:8080")
}
