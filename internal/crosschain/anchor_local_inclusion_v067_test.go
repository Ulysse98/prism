package crosschain

import (
	"strings"
	"testing"
	"time"

	"prism/internal/blockchain"
	"prism/internal/compute"
	"prism/internal/consensus"
	"prism/internal/p2p"
	"prism/internal/usefulwork"
	"prism/internal/wallet"
)

type anchorIncludedFixtureV067 struct {
	candidate anchorProofFixtureV067
	chain     *blockchain.Blockchain
	pos       *consensus.ProofOfStake
}

func newAnchorIncludedFixtureV067(t *testing.T) anchorIncludedFixtureV067 {
	t.Helper()
	validator, err := wallet.New()
	if err != nil {
		t.Fatal(err)
	}
	worker, err := wallet.New()
	if err != nil {
		t.Fatal(err)
	}
	verifier, err := wallet.New()
	if err != nil {
		t.Fatal(err)
	}
	chain, err := blockchain.NewBlockchain(map[string]uint64{validator.Address: 1_000_000})
	if err != nil {
		t.Fatal(err)
	}
	pos := consensus.NewProofOfStake()
	if err := chain.LockStake(validator.Address, 100); err != nil {
		t.Fatal(err)
	}
	if err := pos.Register(validator.Address, 100); err != nil {
		t.Fatal(err)
	}
	genesis := chain.Blocks[0].Hash
	chainID := p2p.MakeChainID(genesis)
	task, err := usefulwork.NewMLInferenceQuantizedTask(
		2, 3, 3,
		[]int64{1, 2, 3, 3, 2, 1},
		[]int64{1, 2, 0, 0, 1, 2, -1, 1, 1},
		[]int64{1, -1, 0},
	)
	if err != nil {
		t.Fatal(err)
	}
	job, err := compute.NewJob(task, "test-requester", 1, 7)
	if err != nil {
		t.Fatal(err)
	}
	const dest = "eip155:421614"
	const registry = "0x1111111111111111111111111111111111111111"
	artifact := strings.Repeat("c", 64)
	contract, err := usefulwork.NewQuantizedMLComputeTaskContractV066(task, job.ID, chainID, genesis, artifact)
	if err != nil {
		t.Fatal(err)
	}
	proof, err := usefulwork.ExecuteCompute(task, job.ID, chainID, genesis, worker)
	if err != nil {
		t.Fatal(err)
	}
	if err := job.Claim(worker.Address); err != nil {
		t.Fatal(err)
	}
	if err := job.Complete(proof); err != nil {
		t.Fatal(err)
	}
	block, err := chain.AddBlock(nil, []usefulwork.Proof{proof}, validator.Address, pos)
	if err != nil {
		t.Fatal(err)
	}
	if !chain.ValidateChain(pos) {
		t.Fatal("fixture source chain is not valid")
	}
	report, err := usefulwork.SignComputeVerificationV064(proof, verifier)
	if err != nil {
		t.Fatal(err)
	}
	f := anchorProofFixtureV067{
		intent: AnchorIntentV067{
			Version:             AnchorIntentVersionV067,
			SourceChainID:       chainID,
			SourceGenesisHash:   genesis,
			SourceBlockHeight:   block.Height,
			SourceBlockHash:     block.Hash,
			JobID:               job.ID,
			ProofHash:           proof.ID,
			DestinationChainID:  dest,
			DestinationRegistry: registry,
			Nonce:               1,
		},
		job: job, contract: contract, proof: proof, report: report,
		policy: AnchorProofPolicyV067{
			SourceChainID:       chainID,
			SourceGenesisHash:   genesis,
			ApprovedArtifactSHA: artifact,
			ApprovedVerifier:    verifier.Address,
			DestinationChainID:  dest,
			DestinationRegistry: registry,
		},
	}
	return anchorIncludedFixtureV067{candidate: f, chain: chain, pos: pos}
}

func (f anchorIncludedFixtureV067) verify() error {
	c := f.candidate
	return VerifyAnchorLocalInclusionV067(c.intent, c.job, c.contract, c.proof, c.report, c.policy, f.chain, f.pos)
}

func TestAnchorLocalInclusionV067ValidFullChain(t *testing.T) {
	f := newAnchorIncludedFixtureV067(t)
	if err := f.verify(); err != nil {
		t.Fatalf("included signed proof on validated local chain rejected: %v", err)
	}
}

func TestAnchorLocalInclusionV067RejectsAdversarialStates(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*anchorIncludedFixtureV067)
	}{
		{"wrong-height", func(f *anchorIncludedFixtureV067) { f.candidate.intent.SourceBlockHeight++ }},
		{"wrong-block-hash", func(f *anchorIncludedFixtureV067) { f.candidate.intent.SourceBlockHash = strings.Repeat("f", 64) }},
		{"missing-included-block", func(f *anchorIncludedFixtureV067) { f.chain.Blocks = f.chain.Blocks[:1] }},
		{"tampered-proof-signature-on-chain", func(f *anchorIncludedFixtureV067) {
			f.chain.Blocks[1].UsefulWork[0].Signature = "00"
			f.chain.Blocks[1].Hash = blockchain.CalculateHash(f.chain.Blocks[1])
			f.candidate.intent.SourceBlockHash = f.chain.Blocks[1].Hash
		}},
		{"duplicate-included-proof", func(f *anchorIncludedFixtureV067) {
			f.chain.Blocks[1].UsefulWork = append(f.chain.Blocks[1].UsefulWork, f.candidate.proof)
			f.chain.Blocks[1].Hash = blockchain.CalculateHash(f.chain.Blocks[1])
			f.candidate.intent.SourceBlockHash = f.chain.Blocks[1].Hash
		}},
		{"rewritten-valid-block-hash", func(f *anchorIncludedFixtureV067) {
			f.chain.Blocks[1].Timestamp = f.chain.Blocks[1].Timestamp.Add(time.Second)
			f.chain.Blocks[1].Hash = blockchain.CalculateHash(f.chain.Blocks[1])
		}},
		{"invalid-validator-snapshot", func(f *anchorIncludedFixtureV067) { f.pos.Validators[0].Stake++ }},
		{"wrong-genesis-pinned", func(f *anchorIncludedFixtureV067) { f.candidate.policy.SourceGenesisHash = strings.Repeat("a", 64) }},
		{"wrong-local-genesis", func(f *anchorIncludedFixtureV067) { f.chain.Blocks[0].Hash = strings.Repeat("e", 64) }},
		{"height-gap", func(f *anchorIncludedFixtureV067) {
			f.chain.Blocks[1].Height += 1
			f.chain.Blocks[1].Hash = blockchain.CalculateHash(f.chain.Blocks[1])
			f.candidate.intent.SourceBlockHeight = f.chain.Blocks[1].Height
			f.candidate.intent.SourceBlockHash = f.chain.Blocks[1].Hash
		}},
		{"nil-local-chain", func(f *anchorIncludedFixtureV067) { f.chain = nil }},
		{"nil-pos", func(f *anchorIncludedFixtureV067) { f.pos = nil }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newAnchorIncludedFixtureV067(t)
			tc.mutate(&f)
			if err := f.verify(); err == nil {
				t.Fatal("adversarial source state unexpectedly accepted")
			}
		})
	}
}

// Even a valid local chain is only a local view, not finality or an oracle for
// a destination contract. No API capable of paying or anchoring is present.
func TestAnchorLocalInclusionV067RejectsMissingSourceProof(t *testing.T) {
	f := newAnchorIncludedFixtureV067(t)
	// Non-genesis block remains valid but the candidate's claimed proof
	// is no longer on it. A structural hash update cannot make it safe.
	f.chain.Blocks[1].UsefulWork = nil
	f.chain.Blocks[1].Hash = blockchain.CalculateHash(f.chain.Blocks[1])
	f.candidate.intent.SourceBlockHash = f.chain.Blocks[1].Hash
	if err := f.verify(); err == nil {
		t.Fatal("missing source proof accepted")
	}
}
