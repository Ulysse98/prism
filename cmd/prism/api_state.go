package main

import (
	"fmt"

	"prism/internal/blockchain"
	"prism/internal/consensus"
	"prism/internal/storage"
	"prism/internal/wallet"
)

func (api *apiServer) loadState() (
	*blockchain.Blockchain,
	*consensus.ProofOfStake,
	map[string]*wallet.Wallet,
	error,
) {
	chain, pos, wallets, err :=
		storage.Load(api.dataPath)

	if err == nil {
		return chain, pos, wallets, nil
	}

	return storage.LoadPublic(
		api.dataPath,
	)
}

func (api *apiServer) saveState(
	chain *blockchain.Blockchain,
	pos *consensus.ProofOfStake,
	wallets map[string]*wallet.Wallet,
) error {
	if storage.Exists(api.dataPath) {
		return storage.Save(
			api.dataPath,
			chain,
			pos,
			wallets,
		)
	}

	if storage.ExistsPublic(api.dataPath) {
		return storage.SavePublicChain(
			api.dataPath,
			chain,
			pos,
		)
	}

	return fmt.Errorf(
		"Prism API state not found: %s",
		api.dataPath,
	)
}
