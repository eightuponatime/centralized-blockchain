package bench

import (
	"encoding/csv"
	"fmt"
	"os"
	"time"

	"github.com/eightuponatime/centralized-blockchain/internal/blockchain"
	"github.com/eightuponatime/centralized-blockchain/internal/models"
	"github.com/mattn/go-colorable"
	"github.com/rs/zerolog"
	"github.com/shopspring/decimal"
)

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

	recordCounts := []int{100, 200, 300, 400, 500, 600, 700, 800, 900, 1000}
	clientId := 101
	trainerId := 202

	for _, count := range recordCounts {
		logger.Info().Int("records", count).Msg("Running benchmark for record count")

		// Clean up previous data
		os.Remove(filename)
		os.Remove("index.dat")

		// Initialize blockchain
		bc := blockchain.NewBlockchain(filename, secretKey)

		// Generate transactions
		startGen := time.Now()
		for i := 0; i < count; i++ {
			tx := &models.Transaction{
				ClientId:    clientId,
				TrainerId:   trainerId,
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
		logger.Info().
			Int("records", count).
			Dur("duration_ms", time.Since(startGen)).
			Msg("Generated transactions")

		// Benchmark FindBlocksByClientId with index
		start := time.Now()
		blocksWithIndex, err := bc.FindBlocksByClientId(clientId)
		if err != nil {
			return fmt.Errorf("failed to find blocks by clientId with index: %w", err)
		}
		clientIdWithIndexMs := time.Since(start).Milliseconds()

		// Benchmark FindBlocksByClientId without index
		start = time.Now()
		blocksWithoutIndex, err := bc.FindBlocksByClientIdWithoutIndex(clientId)
		if err != nil {
			return fmt.Errorf("failed to find blocks by clientId without index: %w", err)
		}
		clientIdWithoutIndexMs := time.Since(start).Milliseconds()

		// Benchmark FindBlocksByClientTrainerId with index
		start = time.Now()
		blocksWithIndexTrainer, err := bc.FindBlocksByClientTrainerId(clientId, trainerId)
		if err != nil {
			return fmt.Errorf("failed to find blocks by client+trainer with index: %w", err)
		}
		clientTrainerIdWithIndexMs := time.Since(start).Milliseconds()

		// Benchmark FindBlocksByClientTrainerId without index
		start = time.Now()
		blocksWithoutIndexTrainer, err := bc.FindBlocksByClientTrainerIdWithoutIndex(clientId, trainerId)
		if err != nil {
			return fmt.Errorf("failed to find blocks by client+trainer without index: %w", err)
		}
		clientTrainerIdWithoutIndexMs := time.Since(start).Milliseconds()

		// Verify results
		if len(blocksWithIndex) != len(blocksWithoutIndex) || len(blocksWithIndexTrainer) != len(blocksWithoutIndexTrainer) {
			return fmt.Errorf("mismatch in block counts: withIndex=%d, withoutIndex=%d, withIndexTrainer=%d, withoutIndexTrainer=%d",
				len(blocksWithIndex), len(blocksWithoutIndex), len(blocksWithIndexTrainer), len(blocksWithoutIndexTrainer))
		}
		blocksReturned := len(blocksWithIndex)

		// Write results to CSV
		if err := writer.Write([]string{
			fmt.Sprintf("%d", count),
			fmt.Sprintf("%d", clientIdWithIndexMs),
			fmt.Sprintf("%d", clientIdWithoutIndexMs),
			fmt.Sprintf("%d", clientTrainerIdWithIndexMs),
			fmt.Sprintf("%d", clientTrainerIdWithoutIndexMs),
			fmt.Sprintf("%d", blocksReturned),
		}); err != nil {
			return fmt.Errorf("failed to write CSV row: %w", err)
		}

		logger.Info().
			Int("records", count).
			Int("blocks_returned", blocksReturned).
			Int64("client_id_with_index_ms", clientIdWithIndexMs).
			Int64("client_id_without_index_ms", clientIdWithoutIndexMs).
			Int64("client_trainer_id_with_index_ms", clientTrainerIdWithIndexMs).
			Int64("client_trainer_id_without_index_ms", clientTrainerIdWithoutIndexMs).
			Msg("Benchmark completed")
	}

	logger.Info().Str("output_file", outputFile).Msg("Benchmark results saved")
	return nil
}