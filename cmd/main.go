package main

import (
	"github.com/eightuponatime/centralized-blockchain/internal/blockchain"
	"github.com/eightuponatime/centralized-blockchain/internal/handlers"
	"github.com/gin-gonic/gin"
)

func main() {
	bc := blockchain.NewBlockchain("blockchain.dat", "my-secret-key-1234567890")

	transactionHandler := handlers.NewTransactionHandler(bc)

	router := gin.Default()
	router.POST("/createGenesis", transactionHandler.CreateGenesis)
	router.POST("/transactions", transactionHandler.CreateTransaction)

	if err := router.Run("localhost:8082"); err != nil {
		panic(err)
	}
}
