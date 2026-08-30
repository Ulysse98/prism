package blockchain

import (
	"fmt"
	"time"

	"prism/internal/transaction"
)

type Blockchain struct {
	Blocks []Block `json:"blocks"`
}

type State struct {
	Balances map[string]uint64
	Nonces   map[string]uint64
}

func NewBlockchain(
	initialBalances map[string]uint64,
) (*Blockchain, error) {

	genesis, err := CreateGenesisBlock(initialBalances)
	if err != nil {
		return nil, err
	}

	return &Blockchain{
		Blocks: []Block{genesis},
	}, nil
}

func (bc *Blockchain) AddBlock(
	transactions []transaction.Transaction,
) (Block, error) {

	if len(transactions) == 0 {
		return Block{}, fmt.Errorf(
			"cannot create an empty block",
		)
	}

	state, err := bc.GetState()
	if err != nil {
		return Block{}, err
	}

	for _, tx := range transactions {
		if err := transaction.ValidateSigned(tx); err != nil {
			return Block{}, err
		}

		expectedNonce := state.Nonces[tx.From]

		if tx.Nonce != expectedNonce {
			return Block{}, fmt.Errorf(
				"invalid nonce for %s: expected %d, got %d",
				tx.From,
				expectedNonce,
				tx.Nonce,
			)
		}

		if state.Balances[tx.From] < tx.Amount {
			return Block{}, fmt.Errorf(
				"insufficient balance for %s: has %d PRISM, needs %d PRISM",
				tx.From,
				state.Balances[tx.From],
				tx.Amount,
			)
		}

		state.Balances[tx.From] -= tx.Amount
		state.Balances[tx.To] += tx.Amount
		state.Nonces[tx.From]++
	}

	previousBlock := bc.Blocks[len(bc.Blocks)-1]

	blockTransactions := make(
		[]transaction.Transaction,
		len(transactions),
	)

	copy(
		blockTransactions,
		transactions,
	)

	block := Block{
		Height:       previousBlock.Height + 1,
		Timestamp:    time.Now().UTC(),
		PreviousHash: previousBlock.Hash,
		Transactions: blockTransactions,
	}

	block.Hash = CalculateHash(block)

	bc.Blocks = append(
		bc.Blocks,
		block,
	)

	return block, nil
}

func (bc *Blockchain) GetState() (
	State,
	error,
) {

	state := State{
		Balances: make(map[string]uint64),
		Nonces:   make(map[string]uint64),
	}

	for blockIndex, block := range bc.Blocks {
		for _, tx := range block.Transactions {

			if blockIndex == 0 {
				if err := transaction.ValidateGenesis(tx); err != nil {
					return State{}, err
				}

				state.Balances[tx.To] += tx.Amount

				continue
			}

			if err := transaction.ValidateSigned(tx); err != nil {
				return State{}, err
			}

			expectedNonce := state.Nonces[tx.From]

			if tx.Nonce != expectedNonce {
				return State{}, fmt.Errorf(
					"invalid chain nonce for %s",
					tx.From,
				)
			}

			if state.Balances[tx.From] < tx.Amount {
				return State{}, fmt.Errorf(
					"invalid chain: insufficient balance for %s",
					tx.From,
				)
			}

			state.Balances[tx.From] -= tx.Amount
			state.Balances[tx.To] += tx.Amount
			state.Nonces[tx.From]++
		}
	}

	return state, nil
}

func (bc *Blockchain) BalanceOf(
	account string,
) (uint64, error) {

	state, err := bc.GetState()
	if err != nil {
		return 0, err
	}

	return state.Balances[account], nil
}

func (bc *Blockchain) NonceOf(
	account string,
) (uint64, error) {

	state, err := bc.GetState()
	if err != nil {
		return 0, err
	}

	return state.Nonces[account], nil
}

func (bc *Blockchain) ValidateChain() bool {
	if len(bc.Blocks) == 0 {
		return false
	}

	genesis := bc.Blocks[0]

	if genesis.Height != 0 {
		return false
	}

	if genesis.PreviousHash != "0" {
		return false
	}

	if CalculateHash(genesis) != genesis.Hash {
		return false
	}

	for i := 1; i < len(bc.Blocks); i++ {
		current := bc.Blocks[i]
		previous := bc.Blocks[i-1]

		if current.Height != previous.Height+1 {
			return false
		}

		if current.PreviousHash != previous.Hash {
			return false
		}

		if CalculateHash(current) != current.Hash {
			return false
		}
	}

	_, err := bc.GetState()

	return err == nil
}
