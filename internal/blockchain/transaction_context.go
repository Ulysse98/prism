package blockchain

import (
	"fmt"

	"prism/internal/transaction"
)

// ValidateTransactionContext is shared by block replay and the mempool.
// Checking the request's chainId at the HTTP layer alone is insufficient.
func (bc *Blockchain) ValidateTransactionContext(tx transaction.Transaction) error {
	if tx.Version != transaction.VersionV2 {
		return nil // historical v1; ValidateSigned is enforced by callers
	}
	chainID, err := bc.ChainID()
	if err != nil {
		return fmt.Errorf("cannot resolve transaction network: %w", err)
	}
	return transaction.ValidateOnChain(tx, chainID, bc.Blocks[0].Hash)
}
