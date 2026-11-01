package crosschain

import (
	"errors"
	"strings"
	"testing"
)

// realAdmissionCandidateV067 copies the real locally included, independently
// signed PoUW fixture. No verifier seam or mocked successful preflight is used.
func realAdmissionCandidateV067(f anchorIncludedFixtureV067) LocalAnchorCandidateV067 {
	c := f.candidate
	return LocalAnchorCandidateV067{
		Intent: c.intent, Job: c.job, Contract: c.contract,
		Proof: c.proof, Report: c.report,
	}
}

func TestLocalAnchorAdmissionRealCryptoV067PositiveAndReplay(t *testing.T) {
	requireLinuxReplayLabV067(t)
	f, cp := newCheckpointFixtureV067(t)
	candidate := realAdmissionCandidateV067(f)
	dataDir := t.TempDir()
	store, err := NewLocalReplayStoreV067(dataDir)
	if err != nil {
		t.Fatal(err)
	}

	// This invokes the production, fixed cryptographic/inclusion/checkpoint gate.
	if _, err := admitLocalAnchorCandidateV067(store, candidate, f.candidate.policy, cp, f.chain, f.pos); err != nil {
		t.Fatalf("real signed, included, checkpointed proof rejected: %v", err)
	}
	if _, err := store.InspectProofV067(candidate.Intent); err != nil {
		t.Fatalf("committed reservation cannot be inspected: %v", err)
	}
	if _, err := admitLocalAnchorCandidateV067(store, candidate, f.candidate.policy, cp, f.chain, f.pos); !errors.Is(err, ErrLocalReplayReservedV067) {
		t.Fatalf("same-process replay was not rejected as already reserved: %v", err)
	}

	reopened, err := NewLocalReplayStoreV067(dataDir)
	if err != nil {
		t.Fatalf("failed to reopen reservation namespace: %v", err)
	}
	if _, err := reopened.InspectProofV067(candidate.Intent); err != nil {
		t.Fatalf("reservation missing on reopened store: %v", err)
	}
	if _, err := admitLocalAnchorCandidateV067(reopened, candidate, f.candidate.policy, cp, f.chain, f.pos); !errors.Is(err, ErrLocalReplayReservedV067) {
		t.Fatalf("restarted-store replay was not rejected: %v", err)
	}
}

func TestLocalAnchorAdmissionRealCryptoV067DestinationAndNonceBypass(t *testing.T) {
	requireLinuxReplayLabV067(t)
	f, cp := newCheckpointFixtureV067(t)
	candidate := realAdmissionCandidateV067(f)
	store, err := NewLocalReplayStoreV067(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := admitLocalAnchorCandidateV067(store, candidate, f.candidate.policy, cp, f.chain, f.pos); err != nil {
		t.Fatalf("first real admission failed: %v", err)
	}

	// Rebind the *same* valid signed source proof to a different destination and
	// nonce. Even with a matching independent policy, this is the same proof.
	other := candidate
	other.Intent.DestinationChainID = "eip155:10143"
	other.Intent.DestinationRegistry = "0x2222222222222222222222222222222222222222"
	other.Intent.Nonce++
	otherPolicy := f.candidate.policy
	otherPolicy.DestinationChainID = other.Intent.DestinationChainID
	otherPolicy.DestinationRegistry = other.Intent.DestinationRegistry
	if err := VerifyAnchorWithPinnedCheckpointV067(
		other.Intent, other.Job, other.Contract, other.Proof, other.Report,
		otherPolicy, cp, f.chain, f.pos,
	); err != nil {
		t.Fatalf("independently valid alternative intent unexpectedly failed preflight: %v", err)
	}
	if _, err := admitLocalAnchorCandidateV067(store, other, otherPolicy, cp, f.chain, f.pos); !errors.Is(err, ErrLocalReplayReservedV067) {
		t.Fatalf("same-proof destination/nonce bypass was not rejected: %v", err)
	}
}

func TestLocalAnchorAdmissionRealCryptoV067InvalidEvidenceLeavesSlotFree(t *testing.T) {
	requireLinuxReplayLabV067(t)
	cases := []struct {
		name   string
		mutate func(*LocalAnchorCandidateV067, *AnchorProofPolicyV067, *PinnedSourceCheckpointV067)
	}{
		{"forged-proof-id", func(c *LocalAnchorCandidateV067, _ *AnchorProofPolicyV067, _ *PinnedSourceCheckpointV067) {
			c.Proof.ID = strings.Repeat("f", 64)
		}},
		{"wrong-source-block-hash", func(c *LocalAnchorCandidateV067, _ *AnchorProofPolicyV067, _ *PinnedSourceCheckpointV067) {
			c.Intent.SourceBlockHash = strings.Repeat("e", 64)
		}},
		{"wrong-pinned-checkpoint-hash", func(_ *LocalAnchorCandidateV067, _ *AnchorProofPolicyV067, p *PinnedSourceCheckpointV067) {
			p.CheckpointBlockHash = strings.Repeat("f", 64)
		}},
		{"unapproved-verifier", func(_ *LocalAnchorCandidateV067, p *AnchorProofPolicyV067, _ *PinnedSourceCheckpointV067) {
			p.ApprovedVerifier = "untrusted-verifier"
		}},
		{"insufficient-checkpoint-depth", func(_ *LocalAnchorCandidateV067, _ *AnchorProofPolicyV067, p *PinnedSourceCheckpointV067) {
			p.MinSuccessorBlocks = 128
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f, cp := newCheckpointFixtureV067(t)
			valid := realAdmissionCandidateV067(f)
			bad := valid
			policy := f.candidate.policy
			badCheckpoint := cp
			tc.mutate(&bad, &policy, &badCheckpoint)

			store, err := NewLocalReplayStoreV067(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			if _, err := admitLocalAnchorCandidateV067(store, bad, policy, badCheckpoint, f.chain, f.pos); err == nil {
				t.Fatal("invalid signed proof, source evidence or policy was admitted")
			}
			// An invalid attempt must never burn the legitimate proof's slot.
			if _, err := admitLocalAnchorCandidateV067(store, valid, f.candidate.policy, cp, f.chain, f.pos); err != nil {
				t.Fatalf("invalid attempt consumed or poisoned valid proof reservation: %v", err)
			}
		})
	}
}
