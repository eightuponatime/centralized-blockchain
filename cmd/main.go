package main

import (
	"encoding/hex"
	"flag"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/eightuponatime/centralized-blockchain/internal/bench"
	"github.com/eightuponatime/centralized-blockchain/internal/blockchain"
	"github.com/eightuponatime/centralized-blockchain/internal/models"
	"github.com/eightuponatime/centralized-blockchain/internal/transaction"
	"github.com/gin-gonic/gin"
	"github.com/mattn/go-colorable"
	"github.com/rs/zerolog"
	"github.com/shopspring/decimal"
)

func main() {
	logger := zerolog.New(zerolog.ConsoleWriter{Out: colorable.NewColorableStdout()}).With().Timestamp().Logger()

	// Parse command-line flags
	benchFlag := flag.Bool("bench", false, "Run performance benchmark")
	flag.Parse()

	filename := "blockchain.dat"
	secretKey := os.Getenv("SECRET_KEY")
	if secretKey == "" {
		secretKey = "my-secret-key-1234567890"
		logger.Warn().Msg("SECRET_KEY not set, using default")
	}

	// Run benchmark if -bench flag is set
	if *benchFlag {
		logger.Info().Msg("Running benchmark mode")
		if err := bench.RunBenchmark(filename, secretKey, "bench_results.csv"); err != nil {
			logger.Fatal().Err(err).Msg("Benchmark failed")
		}
		os.Exit(0)
	}

	// Normal server mode
	bc := blockchain.NewBlockchain(filename, secretKey)

	valid, err := bc.VerifyChain()
	if err != nil {
		logger.Fatal().Err(err).Msg("Error during initial chain verification")
	}
	if !valid {
		logger.Fatal().Msg("Initial chain verification failed")
	}
	logger.Info().Msg("Initial chain verification successful")

	transactionHandler := transaction.NewTransactionHandler(bc)

	router := gin.Default()

	router.Use(func(c *gin.Context) {
		logger.Info().
			Str("method", c.Request.Method).
			Str("url", c.Request.URL.String()).
			Str("client_ip", c.ClientIP()).
			Msg("Received request")
		c.Next()
	})

	// Endpoint to create a genesis block.
	router.POST("/createGenesis", func(c *gin.Context) {
		var req struct {
			ClientId    int             `json:"clientId"`
			TrainerId   int             `json:"trainerId"`
			Amount      decimal.Decimal `json:"amount"`
			PaymentDate time.Time       `json:"paymentDate"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			logger.Error().Err(err).Msg("Invalid request body")
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		logger.Info().Interface("request", req).Msg("Received /createGenesis request")

		transaction := &models.Transaction{
			ClientId:    req.ClientId,
			TrainerId:   req.TrainerId,
			Amount:      req.Amount,
			PaymentDate: req.PaymentDate,
		}
		block, err := transactionHandler.CreateGenesisBlock(transaction)
		if err != nil {
			logger.Error().Err(err).Msg("Failed to create genesis block")
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		logger.Info().
			Int("index", block.Index).
			Str("hash", hex.EncodeToString(block.Hash)).
			Msg("Genesis block created")
		c.JSON(http.StatusOK, gin.H{
			"message": "Genesis block created",
			"block":   block,
		})
	})

	// Endpoint to add a new transaction block.
	router.POST("/transactions", func(c *gin.Context) {
		var req struct {
			ClientId    int             `json:"clientId"`
			TrainerId   int             `json:"trainerId"`
			Amount      decimal.Decimal `json:"amount"`
			PaymentDate time.Time       `json:"paymentDate"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			logger.Error().Err(err).Msg("Invalid request body")
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		logger.Info().Interface("request", req).Msg("Received /transactions request")

		if req.ClientId <= 0 || req.TrainerId <= 0 || req.Amount.LessThanOrEqual(decimal.Zero) {
			logger.Error().
				Int("client_id", req.ClientId).
				Int("trainer_id", req.TrainerId).
				Str("amount", req.Amount.String()).
				Msg("Validation error for transaction")
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid clientId, trainerId, or amount"})
			return
		}
		transaction := &models.Transaction{
			ClientId:    req.ClientId,
			TrainerId:   req.TrainerId,
			Amount:      req.Amount,
			PaymentDate: req.PaymentDate,
		}
		block, err := transactionHandler.NewTransaction(transaction)
		if err != nil {
			logger.Error().Err(err).Msg("Failed to add transaction")
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		logger.Info().
			Int("index", block.Index).
			Str("hash", hex.EncodeToString(block.Hash)).
			Int("client_id", block.Transaction.ClientId).
			Int("trainer_id", block.Transaction.TrainerId).
			Str("amount", block.Transaction.Amount.String()).
			Str("payment_date", block.Transaction.PaymentDate.Format(time.RFC3339)).
			Msg("Created transaction block")
		c.JSON(http.StatusOK, gin.H{
			"message": "Transaction added",
			"block":   block,
		})
	})

	// Endpoint to find blocks by client ID.
	router.GET("/blocks/client/:clientId", func(c *gin.Context) {
		clientIdStr := c.Param("clientId")
		logger.Info().Str("client_id", clientIdStr).Msg("Received /blocks/client request")
		clientId, err := strconv.Atoi(clientIdStr)
		if err != nil {
			logger.Error().Err(err).Msg("Invalid clientId format")
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid clientId format"})
			return
		}
		blocks, err := bc.FindBlocksByClientId(clientId)
		if err != nil {
			logger.Error().Err(err).Int("client_id", clientId).Msg("Failed to find blocks")
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		logger.Info().Int("count", len(blocks)).Int("client_id", clientId).Msg("Found blocks")
		c.JSON(http.StatusOK, gin.H{
			"blocks": blocks,
			"count":  len(blocks),
		})
	})

	// Endpoint to find blocks by client and trainer IDs.
	router.GET("/blocks/client/:clientId/trainer/:trainerId", func(c *gin.Context) {
		clientIdStr := c.Param("clientId")
		trainerIdStr := c.Param("trainerId")
		logger.Info().
			Str("client_id", clientIdStr).
			Str("trainer_id", trainerIdStr).
			Msg("Received /blocks/client/trainer request")

		clientId, err := strconv.Atoi(clientIdStr)
		if err != nil {
			logger.Error().Err(err).Msg("Invalid clientId format")
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid clientId format"})
			return
		}
		trainerId, err := strconv.Atoi(trainerIdStr)
		if err != nil {
			logger.Error().Err(err).Msg("Invalid trainerId format")
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid trainerId format"})
			return
		}
		blocks, err := bc.FindBlocksByClientTrainerId(clientId, trainerId)
		if err != nil {
			logger.Error().
				Err(err).
				Int("client_id", clientId).
				Int("trainer_id", trainerId).
				Msg("Failed to find blocks")
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		logger.Info().
			Int("count", len(blocks)).
			Int("client_id", clientId).
			Int("trainer_id", trainerId).
			Msg("Found blocks")
		c.JSON(http.StatusOK, gin.H{
			"blocks": blocks,
			"count":  len(blocks),
		})
	})

	// Endpoint to get all blocks.
	router.GET("/blocks", func(c *gin.Context) {
		logger.Info().Msg("Received /blocks request")
		blocks, err := bc.GetAllBlocks()
		if err != nil {
			logger.Error().Err(err).Msg("Failed to get all blocks")
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		blockInfos := make([]map[string]interface{}, len(blocks))
		for i, block := range blocks {
			blockInfos[i] = map[string]interface{}{
				"Index":       block.Index,
				"Hash":        hex.EncodeToString(block.Hash),
				"PrevHash":    hex.EncodeToString(block.PrevHash),
				"Timestamp":   block.Timestamp,
				"HMAC":        hex.EncodeToString(block.HMAC),
				"Transaction": block.Transaction,
			}
		}
		logger.Info().Int("count", len(blocks)).Msg("Retrieved blocks")
		c.JSON(http.StatusOK, gin.H{
			"blocks": blockInfos,
			"count":  len(blocks),
		})
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "8082"
		logger.Warn().Msg("PORT not set, using default 8082")
	}
	logger.Info().Str("port", port).Msg("Starting server")
	if err := router.Run(":" + port); err != nil {
		logger.Fatal().Err(err).Msg("Failed to start server")
	}
}