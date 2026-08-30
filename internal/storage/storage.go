package storage

import (
	"bytes"
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"prism/internal/blockchain"
	"prism/internal/consensus"
	"prism/internal/wallet"
)

const (
	chainFilename   = "chain.json"
	walletsFilename = "wallets.json"
)

type ChainFile struct {
	Blockchain *blockchain.Blockchain `json:"blockchain"`
	Validators []consensus.Validator  `json:"validators"`
}

type WalletRecord struct {
	Name       string `json:"name"`
	Address    string `json:"address"`
	PublicKey  string `json:"public_key"`
	PrivateKey string `json:"private_key"`
}

func Exists(dataDir string) bool {
	chainPath := filepath.Join(
		dataDir,
		chainFilename,
	)

	walletsPath := filepath.Join(
		dataDir,
		walletsFilename,
	)

	_, chainErr := os.Stat(chainPath)
	_, walletErr := os.Stat(walletsPath)

	return chainErr == nil && walletErr == nil
}

func Save(
	dataDir string,
	chain *blockchain.Blockchain,
	pos *consensus.ProofOfStake,
	wallets map[string]*wallet.Wallet,
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

	if !chain.ValidateChain(pos) {
		return fmt.Errorf(
			"refusing to save invalid blockchain",
		)
	}

	if err := os.MkdirAll(
		dataDir,
		0755,
	); err != nil {
		return err
	}

	if err := saveChain(
		dataDir,
		chain,
		pos,
	); err != nil {
		return err
	}

	if err := saveWallets(
		dataDir,
		wallets,
	); err != nil {
		return err
	}

	return nil
}

func Load(
	dataDir string,
) (
	*blockchain.Blockchain,
	*consensus.ProofOfStake,
	map[string]*wallet.Wallet,
	error,
) {

	chain, pos, err := loadChain(
		dataDir,
	)
	if err != nil {
		return nil, nil, nil, err
	}

	wallets, err := loadWallets(
		dataDir,
	)
	if err != nil {
		return nil, nil, nil, err
	}

	if !chain.ValidateChain(pos) {
		return nil, nil, nil, fmt.Errorf(
			"stored blockchain failed validation",
		)
	}

	return chain, pos, wallets, nil
}

func saveChain(
	dataDir string,
	chain *blockchain.Blockchain,
	pos *consensus.ProofOfStake,
) error {

	state := ChainFile{
		Blockchain: chain,
		Validators: pos.Validators,
	}

	data, err := json.MarshalIndent(
		state,
		"",
		"  ",
	)
	if err != nil {
		return err
	}

	path := filepath.Join(
		dataDir,
		chainFilename,
	)

	if err := os.WriteFile(
		path,
		data,
		0600,
	); err != nil {
		return err
	}

	return nil
}

func loadChain(
	dataDir string,
) (
	*blockchain.Blockchain,
	*consensus.ProofOfStake,
	error,
) {

	path := filepath.Join(
		dataDir,
		chainFilename,
	)

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, err
	}

	var state ChainFile

	if err := json.Unmarshal(
		data,
		&state,
	); err != nil {
		return nil, nil, err
	}

	if state.Blockchain == nil {
		return nil, nil, fmt.Errorf(
			"stored blockchain is missing",
		)
	}

	if len(state.Blockchain.Blocks) == 0 {
		return nil, nil, fmt.Errorf(
			"stored blockchain has no blocks",
		)
	}

	if state.Blockchain.LockedStakes == nil {
		state.Blockchain.LockedStakes = make(
			map[string]uint64,
		)
	}

	if len(state.Validators) == 0 {
		return nil, nil, fmt.Errorf(
			"stored validator set is empty",
		)
	}

	pos := &consensus.ProofOfStake{
		Validators: state.Validators,
	}

	return state.Blockchain, pos, nil
}

func saveWallets(
	dataDir string,
	wallets map[string]*wallet.Wallet,
) error {

	if len(wallets) == 0 {
		return fmt.Errorf(
			"no wallets to save",
		)
	}

	names := make(
		[]string,
		0,
		len(wallets),
	)

	for name := range wallets {
		names = append(
			names,
			name,
		)
	}

	sort.Strings(names)

	records := make(
		[]WalletRecord,
		0,
		len(names),
	)

	for _, name := range names {
		currentWallet := wallets[name]

		if currentWallet == nil {
			return fmt.Errorf(
				"wallet %s is nil",
				name,
			)
		}

		record := WalletRecord{
			Name:       name,
			Address:    currentWallet.Address,
			PublicKey:  hex.EncodeToString(currentWallet.PublicKey),
			PrivateKey: hex.EncodeToString(currentWallet.PrivateKey),
		}

		records = append(
			records,
			record,
		)
	}

	data, err := json.MarshalIndent(
		records,
		"",
		"  ",
	)
	if err != nil {
		return err
	}

	path := filepath.Join(
		dataDir,
		walletsFilename,
	)

	if err := os.WriteFile(
		path,
		data,
		0600,
	); err != nil {
		return err
	}

	return nil
}

func loadWallets(
	dataDir string,
) (
	map[string]*wallet.Wallet,
	error,
) {

	path := filepath.Join(
		dataDir,
		walletsFilename,
	)

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var records []WalletRecord

	if err := json.Unmarshal(
		data,
		&records,
	); err != nil {
		return nil, err
	}

	if len(records) == 0 {
		return nil, fmt.Errorf(
			"stored wallet file is empty",
		)
	}

	result := make(
		map[string]*wallet.Wallet,
	)

	for _, record := range records {
		if record.Name == "" {
			return nil, fmt.Errorf(
				"wallet name cannot be empty",
			)
		}

		publicRaw, err := hex.DecodeString(
			record.PublicKey,
		)
		if err != nil {
			return nil, fmt.Errorf(
				"invalid public key for wallet %s: %w",
				record.Name,
				err,
			)
		}

		privateRaw, err := hex.DecodeString(
			record.PrivateKey,
		)
		if err != nil {
			return nil, fmt.Errorf(
				"invalid private key for wallet %s: %w",
				record.Name,
				err,
			)
		}

		if len(publicRaw) != ed25519.PublicKeySize {
			return nil, fmt.Errorf(
				"invalid public key size for wallet %s",
				record.Name,
			)
		}

		if len(privateRaw) != ed25519.PrivateKeySize {
			return nil, fmt.Errorf(
				"invalid private key size for wallet %s",
				record.Name,
			)
		}

		publicKey := ed25519.PublicKey(
			publicRaw,
		)

		privateKey := ed25519.PrivateKey(
			privateRaw,
		)

		derivedPublicKey := privateKey.Public().(ed25519.PublicKey)

		if !bytes.Equal(
			publicKey,
			derivedPublicKey,
		) {
			return nil, fmt.Errorf(
				"private key does not match public key for wallet %s",
				record.Name,
			)
		}

		expectedAddress := wallet.AddressFromPublicKey(
			publicKey,
		)

		if record.Address != expectedAddress {
			return nil, fmt.Errorf(
				"stored address does not match public key for wallet %s",
				record.Name,
			)
		}

		result[record.Name] = &wallet.Wallet{
			Address:    record.Address,
			PublicKey:  publicKey,
			PrivateKey: privateKey,
		}
	}

	return result, nil
}
