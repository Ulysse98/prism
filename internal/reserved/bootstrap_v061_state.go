package reserved

import (
	"fmt"

	"prism/internal/consensus"
)

// ApplyBootstrapV061 applies a fully verified bootstrap to an in-memory
// governance replay state. It does not modify a Blockchain, its ChainConfig,
// any existing block, supply, or wallet balances.
//
// Consensus integration MUST call this only while replaying a dedicated,
// authenticated bootstrap block. The validator stake snapshot, genesis hash,
// and block height must be derived from canonical consensus data, never from
// untrusted request fields. Until then this method is not activated by Prism.
func (state *GovernanceState) ApplyBootstrapV061(
	bootstrap GovernanceBootstrap,
	validators []consensus.Validator,
	canonicalGenesisHash string,
	blockHeight uint64,
) error {
	if state == nil {
		return fmt.Errorf("bootstrap governance state is nil")
	}
	if state.ChainID == "" {
		return fmt.Errorf("bootstrap governance state lacks chain ID")
	}
	if err := state.CurrentPolicy.Validate(); err != nil {
		return fmt.Errorf("existing authority policy is invalid: %w", err)
	}
	if bootstrap.Nonce != 1 {
		return fmt.Errorf("first and only team bootstrap must use nonce 1")
	}
	if len(state.CurrentPolicy.Team) != 0 || state.CurrentPolicy.TeamThreshold != 0 {
		return fmt.Errorf("team authorities already initialized: bootstrap forbidden")
	}
	if err := ValidateGovernanceBootstrap(
		bootstrap,
		validators,
		state.ChainID,
		canonicalGenesisHash,
		blockHeight,
		1,
	); err != nil {
		return fmt.Errorf("bootstrap validator authorization failed: %w", err)
	}

	// Work on a copy and commit the policy only after all checks pass.
	updated := cloneAuthorityPolicy(state.CurrentPolicy)
	updated.Team = append([]string(nil), bootstrap.Authorities...)
	updated.TeamThreshold = bootstrap.Threshold
	if err := updated.Validate(); err != nil {
		return fmt.Errorf("bootstrap creates an invalid authority policy: %w", err)
	}
	state.CurrentPolicy = updated
	return nil
}
