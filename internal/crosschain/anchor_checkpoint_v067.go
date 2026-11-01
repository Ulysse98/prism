package crosschain

import (
	"fmt"

	"prism/internal/blockchain"
	"prism/internal/compute"
	"prism/internal/consensus"
	"prism/internal/usefulwork"
)

// PinnedSourceCheckpointV067 is an operator-supplied, out-of-band source-chain
// checkpoint. The caller MUST authenticate its origin separately. This data
// structure has no cryptographic authentication of its own, and does not prove
// BFT finality or that the local validator set is globally canonical.
// MinSuccessorBlocks is a minimum local block-depth policy, NOT finality.
type PinnedSourceCheckpointV067 struct {
	SourceChainID       string
	SourceGenesisHash   string
	CheckpointHeight    uint64
	CheckpointBlockHash string
	MinSuccessorBlocks  uint64
}

// VerifyAnchorWithPinnedCheckpointV067 is a READ-ONLY, OFFLINE diagnostic gate.
// It combines full local-chain inclusion verification with an independently
// supplied checkpoint pin and a minimum number of successor blocks.
//
// Neither success nor the checkpoint can authorize any cross-chain registration,
// wallet signing, settlement, nonce consumption, escrow release or funds transfer.
// The caller must keep chain and pos immutable during verification.
func VerifyAnchorWithPinnedCheckpointV067(
	intent AnchorIntentV067,
	job compute.Job,
	contract usefulwork.ComputeTaskContractV066,
	proof usefulwork.Proof,
	report usefulwork.ComputeVerificationReportV064,
	policy AnchorProofPolicyV067,
	checkpoint PinnedSourceCheckpointV067,
	chain *blockchain.Blockchain,
	pos *consensus.ProofOfStake,
) error {
	if checkpoint.MinSuccessorBlocks < 1 || checkpoint.MinSuccessorBlocks > 128 {
		return fmt.Errorf("v0.67 checkpoint successor depth must be between 1 and 128")
	}
	if checkpoint.SourceChainID != policy.SourceChainID ||
		checkpoint.SourceGenesisHash != policy.SourceGenesisHash ||
		checkpoint.SourceChainID != intent.SourceChainID ||
		checkpoint.SourceGenesisHash != intent.SourceGenesisHash {
		return fmt.Errorf("v0.67 checkpoint source identity differs from pinned policy")
	}
	if err := validHashV067("checkpoint block hash", checkpoint.CheckpointBlockHash); err != nil {
		return err
	}
	if intent.SourceBlockHeight == 0 ||
		checkpoint.CheckpointHeight < intent.SourceBlockHeight ||
		checkpoint.CheckpointHeight-intent.SourceBlockHeight < checkpoint.MinSuccessorBlocks {
		return fmt.Errorf("v0.67 checkpoint too shallow to satisfy requested local block depth")
	}
	if chain == nil || pos == nil || len(chain.Blocks) < 2 {
		return fmt.Errorf("v0.67 no complete local chain or validator set")
	}
	// This validates the entire local chain and checks exactly one included
	// worker-signed proof against the pinned source identity and PoUW policy.
	if err := VerifyAnchorLocalInclusionV067(intent, job, contract, proof, report, policy, chain, pos); err != nil {
		return fmt.Errorf("v0.67 checkpoint source inclusion rejected: %w", err)
	}
	if checkpoint.CheckpointHeight > chain.Blocks[len(chain.Blocks)-1].Height {
		return fmt.Errorf("v0.67 checkpoint lies beyond local validated tip")
	}
	found := 0
	for _, block := range chain.Blocks {
		if block.Height != checkpoint.CheckpointHeight {
			continue
		}
		found++
		if block.Hash != checkpoint.CheckpointBlockHash {
			return fmt.Errorf("v0.67 local checkpoint block differs from externally pinned hash")
		}
	}
	if found != 1 {
		return fmt.Errorf("v0.67 checkpoint height is absent or duplicated")
	}
	return nil // Local evidence ONLY, not a positive settlement authorization.
}
