package blockchain

import (
	"fmt"
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
	// Use the checkpoint fixed at the activation boundary. A later mutation
	// of LockedStakes must not reweight signatures in a historical block.
	if block.Height != v061CheckpointHeight+1 {
		return fmt.Errorf("v0.61 bootstrap only permitted at checkpoint successor height %d", v061CheckpointHeight+1)
	}
	validators, err := bc.pinnedBootstrapValidatorsForChainV061()
	if err != nil {
		return fmt.Errorf("v0.61 bootstrap checkpoint rejected: %w", err)
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
