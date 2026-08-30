package mempool

import (
	"fmt"

	"prism/internal/blockchain"
	"prism/internal/transaction"
)

type Mempool struct {
	transactions []transaction.Transaction
	ids          map[string]struct{}
}

func New() *Mempool {
	return &Mempool{
		transactions: make([]transaction.Transaction, 0),
		ids:          make(map[string]struct{}),
	}
}

func (mp *Mempool) Add(
	tx transaction.Transaction,
	chain *blockchain.Blockchain,
) error {

	if err := transaction.ValidateSigned(tx); err != nil {
		return fmt.Errorf(
			"invalid transaction: %w",
			err,
		)
	}

	if _, exists := mp.ids[tx.ID]; exists {
		return fmt.Errorf(
			"transaction already exists in mempool",
		)
	}

	state, err := mp.stateWithPending(chain)
	if err != nil {
		return err
	}

	expectedNonce := state.Nonces[tx.From]

	if tx.Nonce != expectedNonce {
		return fmt.Errorf(
			"invalid mempool nonce for %s: expected %d, got %d",
			tx.From,
			expectedNonce,
			tx.Nonce,
		)
	}

	if state.Balances[tx.From] < tx.Amount {
		return fmt.Errorf(
			"insufficient pending balance for %s: has %d PRISM, needs %d PRISM",
			tx.From,
			state.Balances[tx.From],
			tx.Amount,
		)
	}

	mp.transactions = append(
		mp.transactions,
		tx,
	)

	mp.ids[tx.ID] = struct{}{}

	return nil
}

func (mp *Mempool) NextNonce(
	address string,
	chain *blockchain.Blockchain,
) (uint64, error) {

	state, err := mp.stateWithPending(chain)
	if err != nil {
		return 0, err
	}

	return state.Nonces[address], nil
}

func (mp *Mempool) Transactions() []transaction.Transaction {
	result := make(
		[]transaction.Transaction,
		len(mp.transactions),
	)

	copy(result, mp.transactions)

	return result
}

func (mp *Mempool) Count() int {
	return len(mp.transactions)
}

func (mp *Mempool) Clear() {
	mp.transactions = make(
		[]transaction.Transaction,
		0,
	)

	mp.ids = make(
		map[string]struct{},
	)
}

func (mp *Mempool) stateWithPending(
	chain *blockchain.Blockchain,
) (blockchain.State, error) {

	state, err := chain.GetState()
	if err != nil {
		return blockchain.State{}, err
	}

	for _, tx := range mp.transactions {

		if err := transaction.ValidateSigned(tx); err != nil {
			return blockchain.State{}, fmt.Errorf(
				"invalid transaction already in mempool: %w",
				err,
			)
		}

		expectedNonce := state.Nonces[tx.From]

		if tx.Nonce != expectedNonce {
			return blockchain.State{}, fmt.Errorf(
				"invalid pending nonce for %s",
				tx.From,
			)
		}

		if state.Balances[tx.From] < tx.Amount {
			return blockchain.State{}, fmt.Errorf(
				"invalid pending balance for %s",
				tx.From,
			)
		}

		state.Balances[tx.From] -= tx.Amount
		state.Balances[tx.To] += tx.Amount
		state.Nonces[tx.From]++
	}

	return state, nil
}
