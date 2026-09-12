package main

import (
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

	return storage.LoadPublic(api.dataPath)
}
