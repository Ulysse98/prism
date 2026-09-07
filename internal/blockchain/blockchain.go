package blockchain

import (
	"fmt"
	"math"
	"time"

	"prism/internal/consensus"
	"prism/internal/identity"
	"prism/internal/transaction"
	"prism/internal/usefulwork"
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
	genesis, err := CreateGenesisBlock(
		initialBalances,
	)
	if err != nil {
		return nil, err
	}

	return &Blockchain{
		Blocks:       []Block{genesis},
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

	balance, err := bc.BalanceOf(
		address,
	)
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
	balance, err := bc.BalanceOf(
		address,
	)
	if err != nil {
		return 0, err
	}

	locked := bc.LockedStakeOf(
		address,
	)

	if locked > balance {
		return 0, fmt.Errorf(
			"locked stake exceeds balance for %s",
			address,
		)
	}

	return balance - locked, nil
}

// AddBlock creates a normal Prism block containing
// transactions and/or useful work.
func (bc *Blockchain) AddBlock(
	transactions []transaction.Transaction,
	workProofs []usefulwork.Proof,
	proposer string,
	pos *consensus.ProofOfStake,
) (Block, error) {
	return bc.addBlock(
		transactions,
		workProofs,
		nil,
		proposer,
		pos,
	)
}

// AddHumanityBlock creates a Prism block containing
// humanity attestations.
func (bc *Blockchain) AddHumanityBlock(
	attestations []identity.Attestation,
	proposer string,
	pos *consensus.ProofOfStake,
) (Block, error) {
	return bc.addBlock(
		nil,
		nil,
		attestations,
		proposer,
		pos,
	)
}

// addBlock is the common block-production path.
func (bc *Blockchain) addBlock(
	transactions []transaction.Transaction,
	workProofs []usefulwork.Proof,
	attestations []identity.Attestation,
	proposer string,
	pos *consensus.ProofOfStake,
) (Block, error) {

	rewardPolicy := consensus.DefaultRewardPolicy()

	if err := rewardPolicy.Validate(); err != nil {
		return Block{}, fmt.Errorf(
			"invalid reward policy: %w",
			err,
		)
	}

	supplyPolicy :=
		consensus.DefaultSupplyPolicy()

	if err := supplyPolicy.Validate(); err != nil {
		return Block{}, fmt.Errorf(
			"invalid supply policy: %w",
			err,
		)
	}

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

	if len(transactions) == 0 &&
		len(workProofs) == 0 &&
		len(attestations) == 0 {

		return Block{}, fmt.Errorf(
			"cannot create an empty block",
		)
	}

	if err := bc.ValidateValidatorSet(
		pos,
	); err != nil {
		return Block{}, err
	}

	if len(bc.Blocks) == 0 {
		return Block{}, fmt.Errorf(
			"blockchain has no genesis block",
		)
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

	// Validate transactions against current spendable state.
	for _, tx := range transactions {
		if err := transaction.ValidateSigned(
			tx,
		); err != nil {
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

		if state.Balances[tx.To] >
			math.MaxUint64-tx.Amount {

			return Block{}, fmt.Errorf(
				"recipient balance overflow",
			)
		}

		state.Balances[tx.From] -= tx.Amount
		state.Balances[tx.To] += tx.Amount
		state.Nonces[tx.From]++
	}

	// Reconstruct already-consumed consensus objects.
	usedTasks := make(
		map[string]struct{},
	)

	usedHumanAddresses := make(
		map[string]struct{},
	)

	usedNullifierHashes := make(
		map[string]struct{},
	)

	for _, block := range bc.Blocks {
		for _, proof := range block.UsefulWork {
			usedTasks[proof.Task.ID] = struct{}{}
		}

		for _, attestation := range block.Humanity {
			usedHumanAddresses[attestation.Address] =
				struct{}{}

			usedNullifierHashes[attestation.NullifierHash] =
				struct{}{}
		}
	}

	// Validate useful work.
	newTasks := make(
		map[string]struct{},
	)

	for _, proof := range workProofs {
		if err := usefulwork.VerifyProof(
			proof,
		); err != nil {
			return Block{}, fmt.Errorf(
				"invalid useful work proof: %w",
				err,
			)
		}

		if _, exists := usedTasks[proof.Task.ID]; exists {
			return Block{}, fmt.Errorf(
				"useful work task already rewarded: %s",
				proof.Task.ID,
			)
		}

		if _, exists := newTasks[proof.Task.ID]; exists {
			return Block{}, fmt.Errorf(
				"duplicate useful work task in block: %s",
				proof.Task.ID,
			)
		}

		newTasks[proof.Task.ID] = struct{}{}
	}

	// Validate humanity attestations.
	newHumanAddresses := make(
		map[string]struct{},
	)

	newNullifierHashes := make(
		map[string]struct{},
	)

	for _, attestation := range attestations {
		if err := identity.ValidateAttestation(
			attestation,
		); err != nil {
			return Block{}, fmt.Errorf(
				"invalid humanity attestation: %w",
				err,
			)
		}

		if _, exists :=
			usedHumanAddresses[attestation.Address]; exists {

			return Block{}, fmt.Errorf(
				"prism address already humanity verified: %s",
				attestation.Address,
			)
		}

		if _, exists :=
			usedNullifierHashes[attestation.NullifierHash]; exists {

			return Block{}, fmt.Errorf(
				"humanity nullifier already attested",
			)
		}

		if _, exists :=
			newHumanAddresses[attestation.Address]; exists {

			return Block{}, fmt.Errorf(
				"duplicate humanity address in block: %s",
				attestation.Address,
			)
		}

		if _, exists :=
			newNullifierHashes[attestation.NullifierHash]; exists {

			return Block{}, fmt.Errorf(
				"duplicate humanity nullifier in block",
			)
		}

		newHumanAddresses[attestation.Address] =
			struct{}{}

		newNullifierHashes[attestation.NullifierHash] =
			struct{}{}
	}

	emission, err :=
		bc.GetEmissionState()

	if err != nil {
		return Block{}, fmt.Errorf(
			"cannot calculate current emissions: %w",
			err,
		)
	}

	proposerReward, err :=
		consensus.BoundedPoolReward(
			rewardPolicy.ProposerReward,
			emission.ProposerEmission,
			supplyPolicy.ProposerRewardPool,
		)

	if err != nil {
		return Block{}, fmt.Errorf(
			"cannot calculate proposer reward: %w",
			err,
		)
	}

	blockTransactions := append(
		[]transaction.Transaction(nil),
		transactions...,
	)

	blockWork := append(
		[]usefulwork.Proof(nil),
		workProofs...,
	)

	blockHumanity := append(
		[]identity.Attestation(nil),
		attestations...,
	)

	block := Block{
		Height:       nextHeight,
		Timestamp:    time.Now().UTC(),
		PreviousHash: previousBlock.Hash,
		Proposer:     proposer,
		Reward:       proposerReward,
		Transactions: blockTransactions,
		UsefulWork:   blockWork,
		Humanity:     blockHumanity,
	}

	block.Hash = CalculateHash(
		block,
	)

	bc.Blocks = append(
		bc.Blocks,
		block,
	)

	return block, nil
}

// AppendValidatedBlock validates and appends an exact block
// received from another Prism node.
//
// Unlike AddBlock, this method never recreates the block timestamp
// or hash. The received block is validated as part of a candidate
// chain before the local chain is mutated.
func (bc *Blockchain) AppendValidatedBlock(
	block Block,
	pos *consensus.ProofOfStake,
) error {
	if bc == nil {
		return fmt.Errorf(
			"blockchain cannot be nil",
		)
	}

	if pos == nil {
		return fmt.Errorf(
			"proof of stake engine cannot be nil",
		)
	}

	if len(bc.Blocks) == 0 {
		return fmt.Errorf(
			"blockchain has no genesis block",
		)
	}

	previous := bc.Blocks[len(bc.Blocks)-1]

	if block.Height != previous.Height+1 {
		return fmt.Errorf(
			"invalid received block height: expected %d, got %d",
			previous.Height+1,
			block.Height,
		)
	}

	if block.PreviousHash != previous.Hash {
		return fmt.Errorf(
			"received block does not extend local tip",
		)
	}

	if block.Hash == "" {
		return fmt.Errorf(
			"received block hash cannot be empty",
		)
	}

	if CalculateHash(block) != block.Hash {
		return fmt.Errorf(
			"invalid received block hash",
		)
	}

	candidateBlocks := make(
		[]Block,
		len(bc.Blocks),
		len(bc.Blocks)+1,
	)

	copy(
		candidateBlocks,
		bc.Blocks,
	)

	candidateBlocks = append(
		candidateBlocks,
		block,
	)

	lockedStakes := make(
		map[string]uint64,
		len(bc.LockedStakes),
	)

	for address, amount := range bc.LockedStakes {
		lockedStakes[address] = amount
	}

	candidate := &Blockchain{
		Blocks:       candidateBlocks,
		LockedStakes: lockedStakes,
	}

	if !candidate.ValidateChain(pos) {
		return fmt.Errorf(
			"received block failed chain validation",
		)
	}

	bc.Blocks = candidateBlocks

	return nil
}

// IsVerified reports whether an address has a valid
// humanity attestation recorded in the Prism blockchain.
func (bc *Blockchain) IsVerified(
	address string,
) bool {
	if address == "" || len(bc.Blocks) == 0 {
		return false
	}

	lastBlock := bc.Blocks[len(bc.Blocks)-1]

	return bc.IsVerifiedAtHeight(
		address,
		lastBlock.Height,
	)
}

// IsVerifiedAtHeight reports whether an address had a valid
// humanity attestation recorded at or before the given block height.
//
// This prevents Proof of Useful Participation from retroactively
// rewarding activity performed before humanity verification.
func (bc *Blockchain) IsVerifiedAtHeight(
	address string,
	height uint64,
) bool {
	if address == "" {
		return false
	}

	for _, block := range bc.Blocks {
		if block.Height > height {
			break
		}

		for _, attestation := range block.Humanity {
			if attestation.Address != address {
				continue
			}

			if identity.ValidateAttestation(
				attestation,
			) == nil {
				return true
			}
		}
	}

	return false
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

// GetState deterministically rebuilds Prism balances,
// nonces, PoS rewards, PoUW rewards and humanity state
// from the blockchain.
func (bc *Blockchain) GetState() (
	State,
	error,
) {

	rewardPolicy := consensus.DefaultRewardPolicy()

	if err := rewardPolicy.Validate(); err != nil {
		return State{}, fmt.Errorf(
			"invalid reward policy: %w",
			err,
		)
	}

	supplyPolicy :=
		consensus.DefaultSupplyPolicy()

	if err := supplyPolicy.Validate(); err != nil {
		return State{}, fmt.Errorf(
			"invalid supply policy: %w",
			err,
		)
	}

	state := State{
		Balances: make(map[string]uint64),
		Nonces:   make(map[string]uint64),
	}

	usedTasks := make(
		map[string]struct{},
	)

	usedHumanAddresses := make(
		map[string]struct{},
	)

	usedNullifierHashes := make(
		map[string]struct{},
	)

	usedParticipationClaims := make(
		map[participationClaimKey]struct{},
	)

	var proposerEmission uint64
	var usefulWorkEmission uint64
	var participationEmission uint64

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

			if len(block.UsefulWork) != 0 {
				return State{}, fmt.Errorf(
					"genesis block cannot contain useful work",
				)
			}

			if len(block.Humanity) != 0 {
				return State{}, fmt.Errorf(
					"genesis block cannot contain humanity attestations",
				)
			}

			if len(block.ParticipationClaims) != 0 {
				return State{}, fmt.Errorf(
					"genesis block cannot contain participation claims",
				)
			}

			if len(block.ReservedAuthorizations) != 0 {
				return State{}, fmt.Errorf(
					"genesis block cannot contain reserved authorizations",
				)
			}

			for _, tx := range block.Transactions {
				if err := transaction.ValidateGenesis(
					tx,
				); err != nil {
					return State{}, err
				}

				if state.Balances[tx.To] >
					math.MaxUint64-tx.Amount {

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

		if len(block.ReservedAuthorizations) != 0 {
			return State{}, fmt.Errorf(
				"reserved authorizations are not activated in consensus",
			)
		}

		expectedProposerReward, err :=
			consensus.BoundedPoolReward(
				rewardPolicy.ProposerReward,
				proposerEmission,
				supplyPolicy.ProposerRewardPool,
			)

		if err != nil {
			return State{}, fmt.Errorf(
				"invalid proposer emission in block %d: %w",
				blockIndex,
				err,
			)
		}

		if block.Reward != expectedProposerReward {
			return State{}, fmt.Errorf(
				"invalid reward in block %d: expected %d, got %d",
				blockIndex,
				expectedProposerReward,
				block.Reward,
			)
		}

		// Transactions, Useful Work, Humanity or PoUP claims
		// can make a normal block non-empty.
		if len(block.Transactions) == 0 &&
			len(block.UsefulWork) == 0 &&
			len(block.Humanity) == 0 &&
			len(block.ParticipationClaims) == 0 {

			return State{}, fmt.Errorf(
				"empty normal block at height %d",
				block.Height,
			)
		}

		// Transactions.
		for _, tx := range block.Transactions {
			if err := transaction.ValidateSigned(
				tx,
			); err != nil {
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

			if state.Balances[tx.To] >
				math.MaxUint64-tx.Amount {

				return State{}, fmt.Errorf(
					"recipient balance overflow",
				)
			}

			state.Balances[tx.From] -= tx.Amount
			state.Balances[tx.To] += tx.Amount
			state.Nonces[tx.From]++
		}

		// Useful Work.
		for _, proof := range block.UsefulWork {
			if err := usefulwork.VerifyProof(
				proof,
			); err != nil {
				return State{}, fmt.Errorf(
					"invalid useful work in block %d: %w",
					blockIndex,
					err,
				)
			}

			if _, exists :=
				usedTasks[proof.Task.ID]; exists {

				return State{}, fmt.Errorf(
					"useful work task rewarded more than once",
				)
			}

			usedTasks[proof.Task.ID] = struct{}{}

			workReward, err :=
				consensus.BoundedPoolReward(
					rewardPolicy.UsefulWorkReward,
					usefulWorkEmission,
					supplyPolicy.UsefulWorkRewardPool,
				)

			if err != nil {
				return State{}, fmt.Errorf(
					"invalid useful work emission in block %d: %w",
					blockIndex,
					err,
				)
			}

			if state.Balances[proof.Worker] >
				math.MaxUint64-workReward {

				return State{}, fmt.Errorf(
					"useful work reward overflow",
				)
			}

			state.Balances[proof.Worker] +=
				workReward

			usefulWorkEmission +=
				workReward
		}

		// Humanity attestations.
		for _, attestation := range block.Humanity {
			if err := identity.ValidateAttestation(
				attestation,
			); err != nil {
				return State{}, fmt.Errorf(
					"invalid humanity attestation in block %d: %w",
					blockIndex,
					err,
				)
			}

			if _, exists :=
				usedHumanAddresses[attestation.Address]; exists {

				return State{}, fmt.Errorf(
					"humanity address attested more than once: %s",
					attestation.Address,
				)
			}

			if _, exists :=
				usedNullifierHashes[attestation.NullifierHash]; exists {

				return State{}, fmt.Errorf(
					"humanity nullifier attested more than once",
				)
			}

			usedHumanAddresses[attestation.Address] =
				struct{}{}

			usedNullifierHashes[attestation.NullifierHash] =
				struct{}{}
		}

		// Proof of Useful Participation claims.
		//
		// Claims are consensus-validated and deterministically
		// credited to balances here.
		for _, claim := range block.ParticipationClaims {
			if err := bc.validateParticipationClaimWithEmission(
				claim,
				block.Height,
				rewardPolicy,
				participationEmission,
				supplyPolicy,
			); err != nil {
				return State{}, fmt.Errorf(
					"invalid participation claim in block %d: %w",
					blockIndex,
					err,
				)
			}

			key := participationClaimKey{
				Address: claim.Address,
				Period:  claim.Period,
			}

			if _, exists :=
				usedParticipationClaims[key]; exists {

				return State{}, fmt.Errorf(
					"participation reward already claimed for address %s period %d",
					claim.Address,
					claim.Period,
				)
			}

			if err := creditParticipationReward(
				&state,
				claim,
			); err != nil {
				return State{}, fmt.Errorf(
					"participation reward credit failed in block %d: %w",
					blockIndex,
					err,
				)
			}

			if participationEmission >
				math.MaxUint64-claim.Amount {

				return State{}, fmt.Errorf(
					"participation emission overflow",
				)
			}

			participationEmission +=
				claim.Amount

			usedParticipationClaims[key] =
				struct{}{}
		}

		// PoS proposer reward.
		if state.Balances[block.Proposer] >
			math.MaxUint64-block.Reward {

			return State{}, fmt.Errorf(
				"block reward overflow",
			)
		}

		state.Balances[block.Proposer] +=
			block.Reward

		proposerEmission +=
			block.Reward
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
	rewardPolicy := consensus.DefaultRewardPolicy()

	if err := rewardPolicy.Validate(); err != nil {
		return false
	}

	supplyPolicy :=
		consensus.DefaultSupplyPolicy()

	if err := supplyPolicy.Validate(); err != nil {
		return false
	}

	if len(bc.Blocks) == 0 {
		return false
	}

	if err := bc.ValidateValidatorSet(
		pos,
	); err != nil {
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

	if len(genesis.UsefulWork) != 0 {
		return false
	}

	if len(genesis.Humanity) != 0 {
		return false
	}

	if len(genesis.ParticipationClaims) != 0 {
		return false
	}

	if CalculateHash(genesis) != genesis.Hash {
		return false
	}

	var proposerEmission uint64

	for i := 1; i < len(bc.Blocks); i++ {
		current := bc.Blocks[i]
		previous := bc.Blocks[i-1]

		if current.Height != previous.Height+1 {
			return false
		}

		if current.PreviousHash != previous.Hash {
			return false
		}

		if current.Proposer == "" ||
			current.Proposer == "GENESIS" {

			return false
		}

		expectedProposerReward, err :=
			consensus.BoundedPoolReward(
				rewardPolicy.ProposerReward,
				proposerEmission,
				supplyPolicy.ProposerRewardPool,
			)

		if err != nil {
			return false
		}

		if current.Reward != expectedProposerReward {
			return false
		}

		proposerEmission +=
			current.Reward

		// Transactions, Useful Work, Humanity OR PoUP claims
		// can make a normal block non-empty.
		if len(current.Transactions) == 0 &&
			len(current.UsefulWork) == 0 &&
			len(current.Humanity) == 0 &&
			len(current.ParticipationClaims) == 0 {

			return false
		}

		expectedProposer, err := pos.SelectProposer(
			previous.Hash,
			current.Height,
		)
		if err != nil {
			return false
		}

		if current.Proposer !=
			expectedProposer.Address {

			return false
		}

		if CalculateHash(current) !=
			current.Hash {

			return false
		}
	}

	// GetState also validates transactions,
	// Useful Work and Humanity attestations.
	if _, err := bc.GetState(); err != nil {
		return false
	}

	if _, err := bc.GetSpendableState(); err != nil {
		return false
	}

	return true
}
