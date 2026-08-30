package blockchain

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"prism/internal/transaction"
)

type Block struct {
	Height       uint64                    `json:"height"`
	Timestamp    time.Time                 `json:"timestamp"`
	PreviousHash string                    `json:"previous_hash"`
	Transactions []transaction.Transaction `json:"transactions"`
	Hash         string                    `json:"hash"`
}

func CalculateHash(block Block) string {
	transactionData, err := json.Marshal(
		block.Transactions,
	)
	if err != nil {
		panic(err)
	}

	payload := fmt.Sprintf(
		"%d|%s|%s|%s",
		block.Height,
		block.Timestamp.UTC().Format(time.RFC3339Nano),
		block.PreviousHash,
		string(transactionData),
	)

	hash := sha256.Sum256([]byte(payload))

	return hex.EncodeToString(hash[:])
}

func CreateGenesisBlock(
	initialBalances map[string]uint64,
) (Block, error) {

	accounts := make(
		[]string,
		0,
		len(initialBalances),
	)

	for account := range initialBalances {
		accounts = append(accounts, account)
	}

	sort.Strings(accounts)

	transactions := make(
		[]transaction.Transaction,
		0,
	)

	for _, account := range accounts {
		if account == "" {
			return Block{}, fmt.Errorf(
				"genesis account cannot be empty",
			)
		}

		if account == "GENESIS" {
			return Block{}, fmt.Errorf(
				"GENESIS is a reserved account",
			)
		}

		amount := initialBalances[account]

		if amount == 0 {
			continue
		}

		tx := transaction.NewGenesis(
			account,
			amount,
		)

		if err := transaction.ValidateGenesis(tx); err != nil {
			return Block{}, err
		}

		transactions = append(
			transactions,
			tx,
		)
	}

	block := Block{
		Height:       0,
		Timestamp:    time.Now().UTC(),
		PreviousHash: "0",
		Transactions: transactions,
	}

	block.Hash = CalculateHash(block)

	return block, nil
}
