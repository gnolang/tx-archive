package backup

//nolint:revive // See https://github.com/gnolang/gno/issues/1197
import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/gnolang/gno/tm2/pkg/amino"
	"github.com/gnolang/tx-archive/backup/client"
	"github.com/gnolang/tx-archive/log"
	"github.com/gnolang/tx-archive/log/noop"
	"github.com/gnolang/tx-archive/types"

	_ "github.com/gnolang/gno/gno.land/pkg/sdk/vm"
)

// Service is the chain backup service
type Service struct {
	client client.Client
	writer io.Writer
	logger log.Logger

	watchInterval time.Duration // interval for the watch routine
}

// NewService creates a new backup service
func NewService(client client.Client, writer io.Writer, opts ...Option) *Service {
	s := &Service{
		client:        client,
		writer:        writer,
		logger:        noop.New(),
		watchInterval: 1 * time.Second,
	}

	for _, opt := range opts {
		opt(s)
	}

	return s
}

// ExecuteBackup executes the node backup process
func (s *Service) ExecuteBackup(ctx context.Context, cfg Config) error {
	// Verify the config
	if cfgErr := ValidateConfig(cfg); cfgErr != nil {
		return fmt.Errorf("invalid config, %w", cfgErr)
	}

	// Determine the right bound
	toBlock, boundErr := determineRightBound(s.client, cfg.ToBlock)
	if boundErr != nil {
		return fmt.Errorf("unable to determine right bound, %w", boundErr)
	}

	fetchAndWrite := func(block uint64) error {
		txs, txErr := s.client.GetBlockTransactions(block)
		if txErr != nil {
			return fmt.Errorf("unable to fetch block transactions, %w", txErr)
		}

		// Skip empty blocks
		if len(txs) == 0 {
			return nil
		}

		// Save the block transaction data, if any
		for _, tx := range txs {
			data := &types.TxData{
				Tx:       tx,
				BlockNum: block,
			}

			// Write the tx data to the file
			if writeErr := writeTxData(s.writer, data); writeErr != nil {
				return fmt.Errorf("unable to write tx data, %w", writeErr)
			}
		}

		// Log the progress
		logProgress(s.logger, cfg.FromBlock, toBlock, block)

		return nil
	}

	// Gather the chain data from the node
	for block := cfg.FromBlock; block <= toBlock; block++ {
		select {
		case <-ctx.Done():
			s.logger.Info("export procedure stopped")

			return nil
		default:
			if fetchErr := fetchAndWrite(block); fetchErr != nil {
				return fetchErr
			}
		}
	}

	// Check if there needs to be a watcher setup
	if cfg.Watch {
		ticker := time.NewTicker(s.watchInterval)
		defer ticker.Stop()

		lastBlock := toBlock

		for {
			select {
			case <-ctx.Done():
				s.logger.Info("export procedure stopped")

				return nil
			case <-ticker.C:
				// Fetch the latest block from the chain
				latest, latestErr := s.client.GetLatestBlockNumber()
				if latestErr != nil {
					return fmt.Errorf("unable to fetch latest block number, %w", latestErr)
				}

				// Check if there have been blocks in the meantime
				if lastBlock == latest {
					continue
				}

				// Catch up to the latest block
				for block := lastBlock + 1; block <= latest; block++ {
					if fetchErr := fetchAndWrite(block); fetchErr != nil {
						return fetchErr
					}
				}

				// Update the last exported block
				lastBlock = latest
			}
		}
	}

	s.logger.Info("Backup complete")

	return nil
}

// determineRightBound determines the
// right bound for the chain backup (block height)
func determineRightBound(
	client client.Client,
	potentialTo *uint64,
) (uint64, error) {
	// Get the latest block height from the chain
	latestBlockNumber, err := client.GetLatestBlockNumber()
	if err != nil {
		return 0, fmt.Errorf("unable to fetch latest block number, %w", err)
	}

	// Check if the chain has the block
	if potentialTo != nil && *potentialTo < latestBlockNumber {
		// Requested right bound is valid, use it
		return *potentialTo, nil
	}

	// Requested right bound is not valid, use the latest block number
	return latestBlockNumber, nil
}

// writeTxData outputs the tx data to the writer
func writeTxData(writer io.Writer, txData *types.TxData) error {
	// Marshal tx data into JSON
	jsonData, err := amino.MarshalJSON(txData)
	if err != nil {
		return fmt.Errorf("unable to marshal JSON data, %w", err)
	}

	// Write the JSON data as a line to the file
	_, err = writer.Write(jsonData)
	if err != nil {
		return fmt.Errorf("unable to write to output, %w", err)
	}

	// Write a newline character to separate JSON objects
	_, err = writer.Write([]byte("\n"))
	if err != nil {
		return fmt.Errorf("unable to write newline output, %w", err)
	}

	return nil
}

// logProgress logs the backup progress
func logProgress(logger log.Logger, from, to, current uint64) {
	total := to - from
	status := (float64(current) - float64(from)) / float64(total) * 100

	logger.Info(
		fmt.Sprintf("Total of %d blocks backed up", current-from+1),
		"total", total+1,
		"from", from,
		"to", to,
		"status", fmt.Sprintf("%.2f%%", status),
	)
}
