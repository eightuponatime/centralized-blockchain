package bench

import (
	"encoding/csv"
	"fmt"
	"math/rand"
	"os"
	"time"

	"github.com/eightuponatime/centralized-blockchain/internal/blockchain"
	"github.com/eightuponatime/centralized-blockchain/internal/models"
	"github.com/mattn/go-colorable"
	"github.com/rs/zerolog"
	"github.com/shopspring/decimal"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

// RunBenchmark runs a performance benchmark for blockchain reading with and without indexes.
func RunBenchmark(filename, secretKey string, outputFile string) error {
	logger := zerolog.New(zerolog.ConsoleWriter{Out: colorable.NewColorableStdout()}).With().Timestamp().Logger()
	logger.Info().Msg("Starting benchmark")

	// Open CSV file for results
	f, err := os.Create(outputFile)
	if err != nil {
		return fmt.Errorf("failed to create output file %s: %w", outputFile, err)
	}
	defer f.Close()

	writer := csv.NewWriter(f)
	defer writer.Flush()

	// Write CSV header
	if err := writer.Write([]string{
		"Records",
		"ClientId_WithIndex_ms",
		"ClientId_WithoutIndex_ms",
		"ClientTrainerId_WithIndex_ms",
		"ClientTrainerId_WithoutIndex_ms",
		"BlocksReturned",
	}); err != nil {
		return fmt.Errorf("failed to write CSV header: %w", err)
	}

	// Test for different numbers of records
	recordCounts := []int{100, 200, 300, 400, 500, 600, 700, 800, 900, 1000, 2000, 5000, 10000}

	// Define 10 target clientId:trainerId pairs (all the same: 1001:2001)
	targetPairs := make([]struct{ clientId, trainerId int }, 10)
	for i := 0; i < 10; i++ {
		targetPairs[i] = struct{ clientId, trainerId int }{
			clientId:  1001,
			trainerId: 2001,
		}
	}

	// Use the target pair for benchmarking
	targetClientId := targetPairs[0].clientId   // 1001
	targetTrainerId := targetPairs[0].trainerId // 2001

	// Set to store the target pair for exclusion in random generation
	targetSet := make(map[string]bool)
	for _, pair := range targetPairs {
		key := fmt.Sprintf("%d:%d", pair.clientId, pair.trainerId)
		targetSet[key] = true
	}

	for _, count := range recordCounts {
		logger.Info().Int("records", count).Msg("Running benchmark for record count")

		// Clean up previous data
		os.Remove(filename)
		os.Remove("index.dat")

		// Initialize blockchain
		bc := blockchain.NewBlockchain(filename, secretKey)

		// Generate transactions
		startGen := time.Now()
		// First, add exactly 10 transactions for the target pair (1001:2001)
		for i, pair := range targetPairs {
			tx := &models.Transaction{
				ClientId:    pair.clientId,
				TrainerId:   pair.trainerId,
				Amount:      decimal.NewFromInt(int64(i + 1)),
				PaymentDate: time.Now(),
			}
			if i == 0 {
				_, err := bc.CreateGenesisBlock(tx)
				if err != nil {
					return fmt.Errorf("failed to create genesis block: %w", err)
				}
			} else {
				_, err := bc.NewBlock(tx)
				if err != nil {
					return fmt.Errorf("failed to create block %d: %w", i+1, err)
				}
			}
		}

		// Now add the remaining transactions with random clientId and trainerId (excluding the target pair)
		for i := 10; i < count; i++ {
			var clientId, trainerId int
			for {
				clientId = rand.Intn(1000) + 1  // Random clientId from 1 to 1000
				trainerId = rand.Intn(1000) + 1 // Random trainerId from 1 to 1000
				key := fmt.Sprintf("%d:%d", clientId, trainerId)
				if !targetSet[key] {
					break // Found a pair that doesn't match the target pair
				}
			}
			tx := &models.Transaction{
				ClientId:    clientId,
				TrainerId:   trainerId,
				Amount:      decimal.NewFromInt(int64(i + 1)),
				PaymentDate: time.Now(),
			}
			_, err := bc.NewBlock(tx)
			if err != nil {
				return fmt.Errorf("failed to create block %d: %w", i+1, err)
			}
		}
		logger.Info().
			Int("records", count).
			Dur("duration_ms", time.Since(startGen)).
			Msg("Generated transactions")

		// Benchmark FindBlocksByClientId with index
		start := time.Now()
		blocksWithIndex, err := bc.FindBlocksByClientId(targetClientId)
		if err != nil {
			return fmt.Errorf("failed to find blocks by clientId with index: %w", err)
		}
		clientIdWithIndexNs := time.Since(start).Nanoseconds()
		clientIdWithIndexMs := float64(clientIdWithIndexNs) / 1e6

		// Benchmark FindBlocksByClientId without index
		start = time.Now()
		blocksWithoutIndex, err := bc.FindBlocksByClientIdWithoutIndex(targetClientId)
		if err != nil {
			return fmt.Errorf("failed to find blocks by clientId without index: %w", err)
		}
		clientIdWithoutIndexNs := time.Since(start).Nanoseconds()
		clientIdWithoutIndexMs := float64(clientIdWithoutIndexNs) / 1e6

		// Benchmark FindBlocksByClientTrainerId with index
		start = time.Now()
		blocksWithIndexTrainer, err := bc.FindBlocksByClientTrainerId(targetClientId, targetTrainerId)
		if err != nil {
			return fmt.Errorf("failed to find blocks by client+trainer with index: %w", err)
		}
		clientTrainerIdWithIndexNs := time.Since(start).Nanoseconds()
		clientTrainerIdWithIndexMs := float64(clientTrainerIdWithIndexNs) / 1e6

		// Benchmark FindBlocksByClientTrainerId without index
		start = time.Now()
		blocksWithoutIndexTrainer, err := bc.FindBlocksByClientTrainerIdWithoutIndex(targetClientId, targetTrainerId)
		if err != nil {
			return fmt.Errorf("failed to find blocks by client+trainer without index: %w", err)
		}
		clientTrainerIdWithoutIndexNs := time.Since(start).Nanoseconds()
		clientTrainerIdWithoutIndexMs := float64(clientTrainerIdWithoutIndexNs) / 1e6

		// Verify results
		if len(blocksWithIndex) != len(blocksWithoutIndex) || len(blocksWithIndexTrainer) != len(blocksWithoutIndexTrainer) {
			return fmt.Errorf("mismatch in block counts: withIndex=%d, withoutIndex=%d, withIndexTrainer=%d, withoutIndexTrainer=%d",
				len(blocksWithIndex), len(blocksWithoutIndex), len(blocksWithIndexTrainer), len(blocksWithoutIndexTrainer))
		}
		blocksReturned := len(blocksWithIndexTrainer) // Should be 10 for the target pair

		// Log distribution for debugging
		logger.Info().
			Int("target_client_id", targetClientId).
			Int("target_trainer_id", targetTrainerId).
			Int("expected_blocks", 10).
			Int("actual_blocks", blocksReturned).
			Int64("client_trainer_id_with_index_ns", clientTrainerIdWithIndexNs).
			Float64("client_trainer_id_with_index_ms", clientTrainerIdWithIndexMs).
			Msg("Distribution check")

		// Write results to CSV with higher precision
		if err := writer.Write([]string{
			fmt.Sprintf("%d", count),
			fmt.Sprintf("%.4f", clientIdWithIndexMs),
			fmt.Sprintf("%.4f", clientIdWithoutIndexMs),
			fmt.Sprintf("%.4f", clientTrainerIdWithIndexMs),
			fmt.Sprintf("%.4f", clientTrainerIdWithoutIndexMs),
			fmt.Sprintf("%d", blocksReturned),
		}); err != nil {
			return fmt.Errorf("failed to write CSV row: %w", err)
		}

		logger.Info().
			Int("records", count).
			Int("blocks_returned", blocksReturned).
			Float64("client_id_with_index_ms", clientIdWithIndexMs).
			Float64("client_id_without_index_ms", clientIdWithoutIndexMs).
			Float64("client_trainer_id_with_index_ms", clientTrainerIdWithIndexMs).
			Float64("client_trainer_id_without_index_ms", clientTrainerIdWithoutIndexMs).
			Msg("Benchmark completed")
	}

	logger.Info().Str("output_file", outputFile).Msg("Benchmark results saved")
	return nil
}