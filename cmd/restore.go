package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"

	"github.com/gnolang/tx-archive/restore"
	"github.com/gnolang/tx-archive/restore/client/http"
	"github.com/gnolang/tx-archive/restore/source"
	"github.com/gnolang/tx-archive/restore/source/legacy"
	"github.com/gnolang/tx-archive/restore/source/standard"
	"github.com/peterbourgon/ff/v3/ffcli"
	"go.uber.org/zap"
)

var errInvalidInputPath = errors.New("invalid file input path")

// restoreCfg is the restore command configuration
type restoreCfg struct {
	inputPath string
	remote    string

	legacyBackup bool
	watch        bool
}

// newRestoreCmd creates the restore command
func newRestoreCmd() *ffcli.Command {
	cfg := &restoreCfg{}

	fs := flag.NewFlagSet("restore", flag.ExitOnError)
	cfg.registerFlags(fs)

	return &ffcli.Command{
		Name:       "restore",
		ShortUsage: "restore [flags]",
		LongHelp:   "Runs the chain restore service",
		FlagSet:    fs,
		Exec:       cfg.exec,
	}
}

// registerFlags registers the restore command flags
func (c *restoreCfg) registerFlags(fs *flag.FlagSet) {
	fs.StringVar(
		&c.inputPath,
		"input-path",
		"",
		"the input path for the JSONL chain data",
	)

	fs.StringVar(
		&c.remote,
		"remote",
		defaultRemoteAddress,
		"the JSON-RPC URL of the chain to be backed up",
	)

	fs.BoolVar(
		&c.legacyBackup,
		"legacy",
		false,
		"flag indicating if the input file is legacy amino JSON",
	)

	fs.BoolVar(
		&c.watch,
		"watch",
		false,
		"flag indicating if the restore should watch incoming tx data",
	)
}

// exec executes the restore command
func (c *restoreCfg) exec(ctx context.Context, _ []string) error {
	// Make sure the remote address is set
	if c.remote == "" {
		return errInvalidRemote
	}

	// Make sure the input file path is valid
	if c.inputPath == "" {
		return errInvalidInputPath
	}

	// Make sure the output file can be overwritten, if it exists
	if _, err := os.Stat(c.inputPath); err != nil {
		// File already exists, and the overwrite flag is not set
		return errOutputFileExists
	}

	// Set up the client
	client := http.NewClient(c.remote)

	// Set up the logger
	zapLogger, loggerErr := zap.NewDevelopment()
	if loggerErr != nil {
		return fmt.Errorf("unable to create logger, %w", loggerErr)
	}

	logger := newCommandLogger(zapLogger)

	// Set up the source
	var (
		src    source.Source
		srcErr error
	)

	if c.legacyBackup {
		src, srcErr = legacy.NewSource(c.inputPath)
	} else {
		src, srcErr = standard.NewSource(c.inputPath)
	}

	if srcErr != nil {
		return fmt.Errorf("unable to create source, %w", srcErr)
	}

	// Set up the source teardown
	teardown := func() {
		if closeErr := src.Close(); closeErr != nil {
			logger.Error(
				"unable to gracefully close source",
				"err",
				closeErr.Error(),
			)
		}
	}

	defer teardown()

	// Create the backup service
	service := restore.NewService(
		client,
		src,
		restore.WithLogger(logger),
	)

	// Run the backup service
	if backupErr := service.ExecuteRestore(ctx, c.watch); backupErr != nil {
		return fmt.Errorf("unable to execute restore, %w", backupErr)
	}

	return nil
}
