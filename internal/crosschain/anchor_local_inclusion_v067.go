package crosschain

import (
	"fmt"
	"reflect"

	"prism/internal/blockchain"
	"prism/internal/compute"
	"prism/internal/consensus"
	"prism/internal/p2p"
	"prism/internal/usefulwork"
)

// VerifyAnchorLocalInclusionV067 verifies source inclusion against a
// locally validated Prism chain and a separately supplied PoS validator set.
// Both must be obtained from independently trusted local state. The caller
// must prevent concurrent mutation of either input throughout this call.
//
// A nil error is only evidence of inclusion in THIS local chain view.
// It establishes neither network-wide canonicality/finality nor permission
// to register, consume a nonce, transfer tokens, or release escrow funds.
func VerifyAnchorLocalInclusionV067(
	intent AnchorIntentV067,
	job compute.Job,
	contract usefulwork.ComputeTaskContractV066,
	proof usefulwork.Proof,
	report usefulwork.ComputeVerificationReportV064,
	policy AnchorProofPolicyV067,
	chain *blockchain.Blockchain,
	pos *consensus.ProofOfStake,
) error {
	if chain == nil || pos == nil || len(chain.Blocks) < 2 {
		return fmt.Errorf("v0.67 missing source chain, validator set, or non-genesis block")
	}
	if err := VerifyAnchorCandidateProofV067(intent, job, contract, proof, report, policy); err != nil {
		return fmt.Errorf("v0.67 source candidate rejected: %w", err)
	}
	genesis := chain.Blocks[0].Hash
	sourceChainID := p2p.MakeChainID(genesis)
	if err := validHashV067("local genesis hash", genesis); err != nil {
		return err
	}
	if genesis != policy.SourceGenesisHash ||
		genesis != intent.SourceGenesisHash ||
		sourceChainID != policy.SourceChainID ||
		sourceChainID != intent.SourceChainID {
		return fmt.Errorf("v0.67 source chain/genesis do not match pinned policy")
	}
	// Full structural, hash-link, proof, and local PoS validation.
	// It does NOT establish globally agreed consensus finality.
	if !chain.ValidateChain(pos) {
		return fmt.Errorf("v0.67 local source chain failed full validation")
	}
	blocksAtHeight := 0
	proofOccurrences := 0
	for _, block := range chain.Blocks {
		if block.Height == intent.SourceBlockHeight {
			blocksAtHeight++
			if block.Hash != intent.SourceBlockHash {
				return fmt.Errorf("v0.67 claimed source block hash differs from local chain")
			}
		}
		for _, found := range block.UsefulWork {
			if found.ID != intent.ProofHash {
				continue
			}
			proofOccurrences++
			if block.Height != intent.SourceBlockHeight ||
				block.Hash != intent.SourceBlockHash ||
				!reflect.DeepEqual(found, proof) {
				return fmt.Errorf("v0.67 source proof differs from claimed block and signed proof")
			}
		}
	}
	if blocksAtHeight != 1 || proofOccurrences != 1 {
		return fmt.Errorf("v0.67 expected exactly one source block and one included proof (got %d, %d)", blocksAtHeight, proofOccurrences)
	}
	return nil
}
