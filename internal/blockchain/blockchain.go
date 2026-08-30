package blockchain

import (
	"fmt"
	"math"
	"time"

	"prism/internal/consensus"
	"prism/internal/transaction"
)

type Blockchain struct {
	Blocks       []Block
	LockedStakes map[string]uint64
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
		Blocks: []Block{
			genesis,
		},
		LockedStakes: make(map[string]uint64),
	}, nil
}

func (bc *Blockchain) LockStake(
	address string,
	amount uint64,
) error {

	if address == "" {
		return fmt.Errorf(
			"stake address cannot be empty",
		)
	}

	if address == "GENESIS" {
		return fmt.Errorf(
			"GENESIS cannot stake",
		)
	}

	if amount == 0 {
		return fmt.Errorf(
			"stake amount must be greater than zero",
		)
	}

	balance, err := bc.BalanceOf(address)
	if err != nil {
		return err
	}

	currentLocked := bc.LockedStakes[address]

	if balance < currentLocked {
		return fmt.Errorf(
			"locked stake exceeds account balance",
		)
	}

	available := balance - currentLocked

	if available < amount {
		return fmt.Errorf(
			"insufficient available balance for stake: has %d PRISM, needs %d PRISM",
			available,
			amount,
		)
	}

	if currentLocked > math.MaxUint64-amount {
		return fmt.Errorf(
			"stake overflow",
		)
	}

	bc.LockedStakes[address] += amount

	return nil
}

func (bc *Blockchain) UnlockStake(
	address string,
	amount uint64,
) error {

	if amount == 0 {
		return fmt.Errorf(
			"unstake amount must be greater than zero",
		)
	}

	currentLocked := bc.LockedStakes[address]

	if currentLocked < amount {
		return fmt.Errorf(
			"cannot unlock %d PRISM: only %d PRISM locked",
			amount,
			currentLocked,
		)
	}

	bc.LockedStakes[address] -= amount

	if bc.LockedStakes[address] == 0 {
		delete(
			bc.LockedStakes,
			address,
		)
	}

	return nil
}

func (bc *Blockchain) LockedStakeOf(
	address string,
) uint64 {

	return bc.LockedStakes[address]
}

func (bc *Blockchain) AvailableBalanceOf(
	address string,
) (uint64, error) {

	balance, err := bc.BalanceOf(address)
	if err != nil {
		return 0, err
	}

	locked := bc.LockedStakeOf(address)

	if locked > balance {
		return 0, fmt.Errorf(
			"locked stake exceeds balance for %s",
			address,
		)
	}

	return balance - locked, nil
}

func (bc *Blockchain) AddBlock(
	transactions []transaction.Transaction,
	proposer string,
	pos *consensus.ProofOfStake,
) (Block, error) {

	if pos == nil {
		return Block{}, fmt.Errorf(
			"proof of stake engine cannot be nil",
		)
	}

	if proposer == "" {
		return Block{}, fmt.Errorf(
			"block proposer cannot be empty",
		)
	}

	if proposer == "GENESIS" {
		return Block{}, fmt.Errorf(
			"GENESIS cannot propose normal blocks",
		)
	}

	if len(transactions) == 0 {
		return Block{}, fmt.Errorf(
			"cannot create an empty block",
		)
	}

	if err := bc.ValidateValidatorSet(pos); err != nil {
		return Block{}, err
	}

	previousBlock := bc.Blocks[len(bc.Blocks)-1]
	nextHeight := previousBlock.Height + 1

	expectedProposer, err := pos.SelectProposer(
		previousBlock.Hash,
		nextHeight,
	)
	if err != nil {
		return Block{}, err
	}

	if proposer != expectedProposer.Address {
		return Block{}, fmt.Errorf(
			"invalid proposer for height %d: expected %s, got %s",
			nextHeight,
			expectedProposer.Address,
			proposer,
		)
	}

	state, err := bc.GetSpendableState()
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
				"insufficient available balance for %s: has %d PRISM, needs %d PRISM",
				tx.From,
				state.Balances[tx.From],
				tx.Amount,
			)
		}

		state.Balances[tx.From] -= tx.Amount
		state.Balances[tx.To] += tx.Amount
		state.Nonces[tx.From]++
	}

	blockTransactions := make(
		[]transaction.Transaction,
		len(transactions),
	)

	copy(
		blockTransactions,
		transactions,
	)

	block := Block{
		Height:       nextHeight,
		Timestamp:    time.Now().UTC(),
		PreviousHash: previousBlock.Hash,
		Proposer:     proposer,
		Reward:       BlockReward,
		Transactions: blockTransactions,
	}

	block.Hash = CalculateHash(block)

	bc.Blocks = append(
		bc.Blocks,
		block,
	)

	return block, nil
}

func (bc *Blockchain) ValidateValidatorSet(
	pos *consensus.ProofOfStake,
) error {

	if pos == nil {
		return fmt.Errorf(
			"proof of stake engine cannot be nil",
		)
	}

	if len(pos.Validators) == 0 {
		return fmt.Errorf(
			"no validators registered",
		)
	}

	for _, validator := range pos.Validators {
		locked := bc.LockedStakeOf(
			validator.Address,
		)

		if locked != validator.Stake {
			return fmt.Errorf(
				"validator stake mismatch for %s: consensus=%d locked=%d",
				validator.Address,
				validator.Stake,
				locked,
			)
		}
	}

	for address, locked := range bc.LockedStakes {
		if locked == 0 {
			continue
		}

		if pos.StakeOf(address) != locked {
			return fmt.Errorf(
				"locked stake has no matching validator for %s",
				address,
			)
		}
	}

	return nil
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

		if blockIndex == 0 {
			if block.Proposer != "GENESIS" {
				return State{}, fmt.Errorf(
					"invalid genesis proposer",
				)
			}

			if block.Reward != 0 {
				return State{}, fmt.Errorf(
					"genesis block cannot contain a reward",
				)
			}

			for _, tx := range block.Transactions {
				if err := transaction.ValidateGenesis(tx); err != nil {
					return State{}, err
				}

				if state.Balances[tx.To] > math.MaxUint64-tx.Amount {
					return State{}, fmt.Errorf(
						"genesis balance overflow",
					)
				}

				state.Balances[tx.To] += tx.Amount
			}

			continue
		}

		if block.Proposer == "" ||
			block.Proposer == "GENESIS" {

			return State{}, fmt.Errorf(
				"invalid proposer in block %d",
				blockIndex,
			)
		}

		if block.Reward != BlockReward {
			return State{}, fmt.Errorf(
				"invalid reward in block %d",
				blockIndex,
			)
		}

		if len(block.Transactions) == 0 {
			return State{}, fmt.Errorf(
				"empty normal block at height %d",
				block.Height,
			)
		}

		for _, tx := range block.Transactions {
			if err := transaction.ValidateSigned(tx); err != nil {
				return State{}, err
			}

			expectedNonce := state.Nonces[tx.From]

			if tx.Nonce != expectedNonce {
				return State{}, fmt.Errorf(
					"invalid chain nonce for %s: expected %d, got %d",
					tx.From,
					expectedNonce,
					tx.Nonce,
				)
			}

			if state.Balances[tx.From] < tx.Amount {
				return State{}, fmt.Errorf(
					"invalid chain: insufficient balance for %s",
					tx.From,
				)
			}

			if state.Balances[tx.To] > math.MaxUint64-tx.Amount {
				return State{}, fmt.Errorf(
					"recipient balance overflow",
				)
			}

			state.Balances[tx.From] -= tx.Amount
			state.Balances[tx.To] += tx.Amount
			state.Nonces[tx.From]++
		}

		if state.Balances[block.Proposer] >
			math.MaxUint64-block.Reward {

			return State{}, fmt.Errorf(
				"block reward overflow",
			)
		}

		state.Balances[block.Proposer] += block.Reward
	}

	return state, nil
}

func (bc *Blockchain) GetSpendableState() (
	State,
	error,
) {

	state, err := bc.GetState()
	if err != nil {
		return State{}, err
	}

	for address, locked := range bc.LockedStakes {
		if state.Balances[address] < locked {
			return State{}, fmt.Errorf(
				"locked stake exceeds balance for %s",
				address,
			)
		}

		state.Balances[address] -= locked
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

func (bc *Blockchain) TotalSupply() (
	uint64,
	error,
) {

	state, err := bc.GetState()
	if err != nil {
		return 0, err
	}

	var total uint64

	for _, balance := range state.Balances {
		if total > math.MaxUint64-balance {
			return 0, fmt.Errorf(
				"total supply overflow",
			)
		}

		total += balance
	}

	return total, nil
}

func (bc *Blockchain) ValidateChain(
	pos *consensus.ProofOfStake,
) bool {

	if len(bc.Blocks) == 0 {
		return false
	}

	if err := bc.ValidateValidatorSet(pos); err != nil {
		return false
	}

	genesis := bc.Blocks[0]

	if genesis.Height != 0 {
		return false
	}

	if genesis.PreviousHash != "0" {
		return false
	}

	if genesis.Proposer != "GENESIS" {
		return false
	}

	if genesis.Reward != 0 {
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

		if current.Proposer == "" {
			return false
		}

		if current.Proposer == "GENESIS" {
			return false
		}

		if current.Reward != BlockReward {
			return false
		}

		if len(current.Transactions) == 0 {
			return false
		}

		expectedProposer, err := pos.SelectProposer(
			previous.Hash,
			current.Height,
		)
		if err != nil {
			return false
		}

		if current.Proposer != expectedProposer.Address {
			return false
		}

		if CalculateHash(current) != current.Hash {
			return false
		}
	}

	if _, err := bc.GetState(); err != nil {
		return false
	}

	if _, err := bc.GetSpendableState(); err != nil {
		return false
	}

	return true
}
