package backup

import (
	"github.com/gnolang/tx-archive/backup/client"
)

type (
	getLatestBlockNumberDelegate func() (uint64, error)
	getBlockTransactionsDelegate func(uint64) (*client.Block, error)
)

type mockClient struct {
	getLatestBlockNumberFn getLatestBlockNumberDelegate
	getBlockTransactionsFn getBlockTransactionsDelegate
}

func (m *mockClient) GetLatestBlockNumber() (uint64, error) {
	if m.getLatestBlockNumberFn != nil {
		return m.getLatestBlockNumberFn()
	}

	return 0, nil
}

func (m *mockClient) GetBlockTransactions(blockNum uint64) (*client.Block, error) {
	if m.getBlockTransactionsFn != nil {
		return m.getBlockTransactionsFn(blockNum)
	}

	return nil, nil
}
