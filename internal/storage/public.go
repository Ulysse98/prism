package storage

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"prism/internal/blockchain"
	"prism/internal/consensus"
	"prism/internal/wallet"
)

const publicWalletsFilename = "api-wallets.json"

type PublicWalletRecord struct {
	Name    string `json:"name"`
	Address string `json:"address"`
}

func ExistsPublic(dataDir string) bool {
	chainPath := filepath.Join(dataDir, chainFilename)
	walletsPath := filepath.Join(dataDir, publicWalletsFilename)

	_, chainErr := os.Stat(chainPath)
	_, walletsErr := os.Stat(walletsPath)

	return chainErr == nil && walletsErr == nil
}

func LoadPublic(
	dataDir string,
) (
	*blockchain.Blockchain,
	*consensus.ProofOfStake,
	map[string]*wallet.Wallet,
	error,
) {
	chain, pos, err := loadChain(dataDir)
	if err != nil {
		return nil, nil, nil, err
	}

	if !chain.ValidateChain(pos) {
		return nil, nil, nil,
			fmt.Errorf("stored blockchain failed validation")
	}

	path := filepath.Join(
		dataDir,
		publicWalletsFilename,
	)

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, nil, err
	}

	var records []PublicWalletRecord

	if err := json.Unmarshal(data, &records); err != nil {
		return nil, nil, nil, err
	}

	if len(records) == 0 {
		return nil, nil, nil,
			fmt.Errorf("public wallet file is empty")
	}

	wallets := make(map[string]*wallet.Wallet)

	for _, record := range records {
		if record.Name == "" {
			return nil, nil, nil,
				fmt.Errorf("public wallet name cannot be empty")
		}

		if record.Address == "" {
			return nil, nil, nil,
				fmt.Errorf(
					"public wallet address cannot be empty: %s",
					record.Name,
				)
		}

		if _, exists := wallets[record.Name]; exists {
			return nil, nil, nil,
				fmt.Errorf(
					"duplicate public wallet name: %s",
					record.Name,
				)
		}

		wallets[record.Name] = &wallet.Wallet{
			Address: record.Address,
		}
	}

	return chain, pos, wallets, nil
}

func SavePublicChain(
	dataDir string,
	chain *blockchain.Blockchain,
	pos *consensus.ProofOfStake,
) error {
	if chain == nil {
		return fmt.Errorf(
			"blockchain cannot be nil",
		)
	}

	if pos == nil {
		return fmt.Errorf(
			"proof of stake engine cannot be nil",
		)
	}

	if !ExistsPublic(dataDir) {
		return fmt.Errorf(
			"public Prism state not found: %s",
			dataDir,
		)
	}

	if !chain.ValidateChain(pos) {
		return fmt.Errorf(
			"refusing to save invalid public blockchain",
		)
	}

	return saveChain(
		dataDir,
		chain,
		pos,
	)
}
