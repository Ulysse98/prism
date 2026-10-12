package blockchain

import (
	"fmt"
	"sort"

	"prism/internal/consensus"
	"prism/internal/reserved"
)

// GovernanceBootstrapActivationHeightV061 is the earliest height a
// v0.61 bootstrap is permitted. This is a CONSENSUS RULE and requires
// validator coordination before deployment. It is not a local config switch.
const GovernanceBootstrapActivationHeightV061 uint64 = 6

func (bc *Blockchain) hasGovernanceBootstrapV061() bool {
	if bc == nil {
		return false
	}
	for _, block := range bc.Blocks {
		if len(block.GovernanceBootstraps) != 0 {
			return true
		}
	}
	return false
}

// A bootstrap block carries only a bootstrap intent and its proposer reward;
// no transfer, grant, authority proposal or other workload can share it.
// This avoids ambiguous ordering at the governance activation boundary.
func standaloneGovernanceBootstrapBlockV061(block Block) bool {
	return len(block.GovernanceBootstraps) == 1 &&
		len(block.Transactions) == 0 &&
		len(block.UsefulWork) == 0 &&
		len(block.Humanity) == 0 &&
		len(block.ParticipationClaims) == 0 &&
		len(block.ReservedAuthorizations) == 0 &&
		len(block.ReservedGrants) == 0 &&
		len(block.ReservedRevocations) == 0 &&
		len(block.AuthorityChanges) == 0 &&
		len(block.AuthorityProposals) == 0 &&
		len(block.AuthorityExecutions) == 0 &&
		len(block.ReservedTransferProposals) == 0 &&
		len(block.ReservedTransferExecutions) == 0
}

// applyGovernanceBootstrapBlockV061 must run during canonical replay.
// Stake is derived ONLY from the chain's locked-stake state and must match
// the PoS validator set through ValidateChain/ValidateValidatorSet.
func (bc *Blockchain) applyGovernanceBootstrapBlockV061(
	block Block,
	governance *reserved.GovernanceState,
) error {
	if bc == nil || governance == nil || len(bc.Blocks) == 0 {
		return fmt.Errorf("bootstrap replay requires canonical chain and governance state")
	}
	if !standaloneGovernanceBootstrapBlockV061(block) {
		return fmt.Errorf("bootstrap block must contain exactly one standalone bootstrap")
	}
	if block.Height < GovernanceBootstrapActivationHeightV061 {
		return fmt.Errorf("bootstrap block before activation height %d", GovernanceBootstrapActivationHeightV061)
	}
	if len(bc.LockedStakes) == 0 {
		return fmt.Errorf("bootstrap has no canonical validator stake")
	}

	// Sorted ordering ensures reproducible validator snapshot encoding.
	keys := make([]string, 0, len(bc.LockedStakes))
	for address := range bc.LockedStakes {
		keys = append(keys, address)
	}
	sort.Strings(keys)
	validators := make([]consensus.Validator, 0, len(keys))
	for _, address := range keys {
		amount := bc.LockedStakes[address]
		if amount == 0 {
			return fmt.Errorf("bootstrap invalid zero locked stake")
		}
		validators = append(validators, consensus.Validator{Address: address, Stake: amount})
	}
	if err := governance.ApplyBootstrapV061(
		block.GovernanceBootstraps[0],
		validators,
		bc.Blocks[0].Hash,
		block.Height,
	); err != nil {
		return fmt.Errorf("invalid v0.61 consensus bootstrap at height %d: %w", block.Height, err)
	}
	return nil
}
