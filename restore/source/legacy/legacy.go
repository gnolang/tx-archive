package legacy

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"

	"github.com/gnolang/gno/tm2/pkg/amino"
	"github.com/gnolang/gno/tm2/pkg/std"
)

type Legacy struct {
	file    *os.File
	scanner *bufio.Scanner
}

// NewLegacySource creates a new legacy amino JSON source
func NewLegacySource(filePath string) (*Legacy, error) {
	// Open the file
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf(
			"unable to open legacy input file, %w",
			err,
		)
	}

	return &Legacy{
		file:    file,
		scanner: bufio.NewScanner(file),
	}, nil
}

func (l *Legacy) Next(ctx context.Context) (*std.Tx, error) {
	for l.scanner.Scan() {
		select {
		case <-ctx.Done():
			return nil, io.EOF
		default:
			// Parse the amino JSON
			var tx std.Tx

			if err := amino.UnmarshalJSON(l.scanner.Bytes(), &tx); err != nil {
				return nil, fmt.Errorf(
					"unable to unmarshal amino JSON, %w",
					err,
				)
			}

			return &tx, nil
		}
	}

	// Check for scanning errors
	if err := l.scanner.Err(); err != nil {
		return nil, fmt.Errorf(
			"unable to read legacy input file, %w",
			err,
		)
	}

	return nil, io.EOF
}

func (l *Legacy) Close() error {
	// Attempt to gracefully close the file
	if closeErr := l.file.Close(); closeErr != nil {
		return fmt.Errorf(
			"unable to gracefully close legacy file, %w",
			closeErr,
		)
	}

	return nil
}
