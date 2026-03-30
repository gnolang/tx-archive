package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/gnolang/gno/gno.land/pkg/gnoland"
	_ "github.com/gnolang/gno/gno.land/pkg/sdk/vm"
	"github.com/gnolang/gno/tm2/pkg/amino"
	bft "github.com/gnolang/gno/tm2/pkg/bft/types"
	"github.com/peterbourgon/ff/v3/ffcli"
)

var (
	errMissingInput   = errors.New("--input is required")
	errMissingOutput  = errors.New("--output is required")
	errMissingChainID = errors.New("--chain-id is required")
)

// genesisAssembleCfg is the genesis-assemble command configuration
type genesisAssembleCfg struct {
	input           string
	output          string
	chainID         string
	initialHeight   int64
	genesisTime     string
	genesisTemplate string
}

// newGenesisAssembleCmd creates the genesis-assemble command
func newGenesisAssembleCmd() *ffcli.Command {
	cfg := &genesisAssembleCfg{}

	fs := flag.NewFlagSet("genesis-assemble", flag.ExitOnError)
	cfg.registerFlags(fs)

	return &ffcli.Command{
		Name:       "genesis-assemble",
		ShortUsage: "genesis-assemble [flags]",
		LongHelp:   "Assembles a genesis.json from a JSONL tx export",
		FlagSet:    fs,
		Exec:       cfg.exec,
	}
}

// registerFlags registers the genesis-assemble command flags
func (c *genesisAssembleCfg) registerFlags(fs *flag.FlagSet) {
	fs.StringVar(
		&c.input,
		"input",
		"",
		"the input JSONL file path (required)",
	)

	fs.StringVar(
		&c.output,
		"output",
		"",
		"the output genesis.json file path (required)",
	)

	fs.StringVar(
		&c.chainID,
		"chain-id",
		"",
		"the chain ID for the new genesis (required)",
	)

	fs.Int64Var(
		&c.initialHeight,
		"initial-height",
		0,
		"the initial block height (0 = infer from max block_height + 1)",
	)

	fs.StringVar(
		&c.genesisTime,
		"genesis-time",
		"",
		"the genesis time in RFC3339 format (default: now)",
	)

	fs.StringVar(
		&c.genesisTemplate,
		"genesis-template",
		"",
		"optional base genesis.json to merge app_state.txs into",
	)
}

// exec executes the genesis-assemble command
func (c *genesisAssembleCfg) exec(_ context.Context, _ []string) error {
	// Validate required flags
	if c.input == "" {
		return errMissingInput
	}

	if c.output == "" {
		return errMissingOutput
	}

	if c.chainID == "" {
		return errMissingChainID
	}

	// Read the JSONL input file
	txs, maxBlockHeight, err := readJSONLTxs(c.input)
	if err != nil {
		return fmt.Errorf("unable to read JSONL input, %w", err)
	}

	// Determine initial height
	initialHeight := c.initialHeight
	if initialHeight == 0 && maxBlockHeight > 0 {
		initialHeight = maxBlockHeight + 1
	}

	// NOTE: GenesisDoc does not yet have an InitialHeight field.
	// When it's added (Jae is working on this), set it here.
	// For now, just log the computed value.
	fmt.Fprintf(os.Stderr, "Computed initial_height: %d (will be used when GenesisDoc supports it)\n", initialHeight)

	// Determine genesis time
	genesisTime := time.Now()
	if c.genesisTime != "" {
		parsed, parseErr := time.Parse(time.RFC3339, c.genesisTime)
		if parseErr != nil {
			return fmt.Errorf("unable to parse genesis time %q, %w", c.genesisTime, parseErr)
		}

		genesisTime = parsed
	}

	// Build or load genesis doc
	var genDoc *bft.GenesisDoc

	if c.genesisTemplate != "" {
		// Load template genesis
		tmplDoc, loadErr := bft.GenesisDocFromFile(c.genesisTemplate)
		if loadErr != nil {
			return fmt.Errorf("unable to load genesis template, %w", loadErr)
		}

		genDoc = tmplDoc
	} else {
		genDoc = &bft.GenesisDoc{}
	}

	// Set chain ID and genesis time
	genDoc.ChainID = c.chainID
	genDoc.GenesisTime = genesisTime

	// Build the app state with txs
	appState := gnoland.GnoGenesisState{
		Txs: txs,
	}

	// If template had an existing app state, try to preserve non-tx fields
	if c.genesisTemplate != "" && genDoc.AppState != nil {
		if existing, ok := genDoc.AppState.(gnoland.GnoGenesisState); ok {
			appState.Balances = existing.Balances
			appState.Auth = existing.Auth
			appState.Bank = existing.Bank
			appState.VM = existing.VM
			appState.Txs = txs // override txs with our export
		}
	}

	genDoc.AppState = appState

	// Marshal and write the output
	genDocBytes, marshalErr := amino.MarshalJSONIndent(genDoc, "", "  ")
	if marshalErr != nil {
		return fmt.Errorf("unable to marshal genesis doc, %w", marshalErr)
	}

	if writeErr := os.WriteFile(c.output, genDocBytes, 0o644); writeErr != nil {
		return fmt.Errorf("unable to write genesis file, %w", writeErr)
	}

	fmt.Fprintf(os.Stderr, "Genesis file written to %s (%d txs, chain_id=%s)\n", c.output, len(txs), c.chainID)

	return nil
}

// readJSONLTxs reads TxWithMetadata entries from a JSONL file.
// Returns the collected txs and the maximum block height found.
func readJSONLTxs(path string) ([]gnoland.TxWithMetadata, int64, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, 0, fmt.Errorf("unable to open input file, %w", err)
	}
	defer file.Close()

	var (
		txs            []gnoland.TxWithMetadata
		maxBlockHeight int64
	)

	scanner := bufio.NewScanner(file)

	// Increase scanner buffer for large lines
	scanner.Buffer(make([]byte, 0, 64*1024), 10*1024*1024)

	lineNum := 0
	for scanner.Scan() {
		lineNum++

		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var tx gnoland.TxWithMetadata
		if unmarshalErr := amino.UnmarshalJSON(line, &tx); unmarshalErr != nil {
			return nil, 0, fmt.Errorf("unable to unmarshal tx at line %d, %w", lineNum, unmarshalErr)
		}

		// Track max block height from metadata
		if tx.Metadata != nil && tx.Metadata.BlockHeight > maxBlockHeight {
			maxBlockHeight = tx.Metadata.BlockHeight
		}

		txs = append(txs, tx)
	}

	if scanErr := scanner.Err(); scanErr != nil {
		return nil, 0, fmt.Errorf("unable to read input file, %w", scanErr)
	}

	return txs, maxBlockHeight, nil
}
