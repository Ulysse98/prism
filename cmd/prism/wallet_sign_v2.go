package main

import (
	"fmt"

	"prism/internal/blockchain"
	"prism/internal/transaction"
	"prism/internal/wallet"
)

// signLocalWalletTransferV2 signs a payment using a wallet already held by the
// local node. It is NOT an HTTP signing endpoint: no private key comes from or
// is returned to a remote caller. Faucet and compute bounty payments share the
// same network-bound signature scheme used by external wallet transfers.
func signLocalWalletTransferV2(
	chain *blockchain.Blockchain,
	sender *wallet.Wallet,
	recipient string,
	amount uint64,
	nonce uint64,
) (transaction.Transaction, error) {
	if chain == nil {
		return transaction.Transaction{}, fmt.Errorf("wallet signing requires a blockchain")
	}
	if sender == nil {
		return transaction.Transaction{}, fmt.Errorf("wallet signing requires a local sender")
	}
	chainID, err := chain.ChainID()
	if err != nil {
		return transaction.Transaction{}, fmt.Errorf("cannot resolve wallet signing network: %w", err)
	}
	genesisHash := chain.Blocks[0].Hash
	tx, err := transaction.NewV2(
		sender.Address,
		recipient,
		amount,
		nonce,
		sender.PublicKeyHex(),
		chainID,
		genesisHash,
	)
	if err != nil {
		return transaction.Transaction{}, fmt.Errorf("cannot construct v2 transfer: %w", err)
	}
	if err := tx.Sign(sender.PrivateKey); err != nil {
		return transaction.Transaction{}, fmt.Errorf("cannot sign v2 transfer: %w", err)
	}
	if err := chain.ValidateTransactionContext(tx); err != nil {
		return transaction.Transaction{}, fmt.Errorf("signed v2 transfer network mismatch: %w", err)
	}
	return tx, nil
}
