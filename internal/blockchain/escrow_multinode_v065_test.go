package blockchain

import (
	"reflect"
	"testing"

	"prism/internal/compute"
	"prism/internal/consensus"
	"prism/internal/escrow"
	"prism/internal/usefulwork"
)

func checkEscrowPeerV065(t *testing.T, source, peer *Blockchain, pos *consensus.ProofOfStake, jobID string, refund bool) {
	t.Helper()
	if !source.ValidateChain(pos) || !peer.ValidateChain(pos) {
		t.Fatal("source or peer failed consensus validation")
	}
	if len(source.Blocks) != len(peer.Blocks) {
		t.Fatal("peer has a different block count")
	}
	for i := range source.Blocks {
		if source.Blocks[i].Hash != peer.Blocks[i].Hash {
			t.Fatalf("different block hash at height %d", i)
		}
	}
	for _, check := range []struct {
		name string
		get  func(*Blockchain) (State, error)
	}{
		{"ledger", (*Blockchain).GetState},
		{"spendable", (*Blockchain).GetSpendableState},
	} {
		a, err := check.get(source)
		if err != nil {
			t.Fatal(err)
		}
		b, err := check.get(peer)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(a, b) {
			t.Fatalf("%s differs: source=%+v peer=%+v", check.name, a, b)
		}
	}
	a, err := source.GetEscrowRecordV065(jobID)
	if err != nil {
		t.Fatal(err)
	}
	b, err := peer.GetEscrowRecordV065(jobID)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(a, b) || (refund && b.Status != escrow.Refunded) || (!refund && b.Status != escrow.Released) {
		t.Fatalf("escrow state differs: source=%+v peer=%+v", a, b)
	}
}

func TestEscrowV065PeerReplayReleaseRefundAndForgery(t *testing.T) {
	for _, tc := range []struct {
		name   string
		expiry uint64
		refund bool
	}{
		{"release", 10, false},
		{"refund", 3, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			source, alice, bob, task := escrowLifecycleFixtureV065(t, tc.expiry)
			pos := escrowTestPoSV065(t, alice)
			peer := &Blockchain{
				Blocks:       append([]Block(nil), source.Blocks[:1]...),
				LockedStakes: map[string]uint64{alice.Address: 10},
				Config:       source.Config,
			}
			jobID := escrow.CalculateFundedJobID(task.ID, alice.Address, 10, 55)
			chainID, err := source.ChainID()
			if err != nil {
				t.Fatal(err)
			}
			claim, err := compute.SignClaimAuthorization(jobID, chainID, source.Blocks[0].Hash, bob)
			if err != nil {
				t.Fatal(err)
			}
			claimBlock, err := source.AddEscrowClaimBlockV065(claim, alice.Address, pos)
			if err != nil {
				t.Fatal(err)
			}
			if tc.refund {
				if _, err := source.AddEscrowRefundBlockV065(jobID, alice.Address, pos); err != nil {
					t.Fatal(err)
				}
			} else {
				proof, err := usefulwork.ExecuteCompute(task, jobID, chainID, source.Blocks[0].Hash, bob)
				if err != nil {
					t.Fatal(err)
				}
				if _, err := source.AddEscrowReleaseBlockV065(proof, alice.Address, pos); err != nil {
					t.Fatal(err)
				}
			}
			if err := peer.AppendValidatedBlock(source.Blocks[1], pos); err != nil {
				t.Fatal(err)
			}
			forged := claimBlock
			forged.EscrowClaims = append([]compute.ClaimAuthorization(nil), claimBlock.EscrowClaims...)
			forged.EscrowClaims[0].Worker = alice.Address
			forged.Hash = CalculateHash(forged)
			if err := peer.AppendValidatedBlock(forged, pos); err == nil {
				t.Fatal("rehashed claim with forged worker was accepted")
			}
			if len(peer.Blocks) != 2 {
				t.Fatal("rejected claim mutated peer chain")
			}
			if err := peer.AppendValidatedBlock(claimBlock, pos); err != nil {
				t.Fatal(err)
			}
			if !tc.refund {
				release := source.Blocks[len(source.Blocks)-1]
				forged = release
				forged.EscrowReleases = append([]usefulwork.Proof(nil), release.EscrowReleases...)
				forged.EscrowReleases[0].Result++
				forged.Hash = CalculateHash(forged)
				if err := peer.AppendValidatedBlock(forged, pos); err == nil {
					t.Fatal("rehashed release with forged result was accepted")
				}
				if len(peer.Blocks) != 3 {
					t.Fatal("rejected release mutated peer chain")
				}
			}
			if err := peer.AppendValidatedBlock(source.Blocks[len(source.Blocks)-1], pos); err != nil {
				t.Fatal(err)
			}
			checkEscrowPeerV065(t, source, peer, pos, jobID, tc.refund)
		})
	}
}
