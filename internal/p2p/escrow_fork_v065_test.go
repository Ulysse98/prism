package p2p

import (
	"bytes"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"prism/internal/blockchain"
	"prism/internal/escrow"
	"prism/internal/storage"
	"prism/internal/wallet"
)

func TestEscrowV065P2PForkBetweenIsolatedValidators(t *testing.T) {
	base, pos, alice, bob, proof, jobID :=
		escrowCompetingFixtureV065(t, 10)

	branchA := cloneEscrowTipV065(base)
	branchB := cloneEscrowTipV065(base)

	blockA, err := branchA.AddEscrowReleaseBlockV065(
		proof, alice.Address, pos,
	)
	if err != nil {
		t.Fatal(err)
	}

	time.Sleep(3 * time.Millisecond)

	blockB, err := branchB.AddEscrowReleaseBlockV065(
		proof, alice.Address, pos,
	)
	if err != nil {
		t.Fatal(err)
	}

	if blockA.Height != blockB.Height ||
		blockA.Hash == blockB.Hash {
		t.Fatal("expected distinct competing blocks at same height")
	}

	if !branchA.ValidateChain(pos) || !branchB.ValidateChain(pos) {
		t.Fatal("one candidate branch is invalid")
	}

	dirA := t.TempDir()
	dirB := t.TempDir()

	newReceiver := func(name, dir string) *Server {
		return NewServer(
			name,
			"127.0.0.1:0",
			dir,
			cloneEscrowTipV065(base),
			pos,
			map[string]*wallet.Wallet{
				"Alice": alice,
				"Bob":   bob,
			},
		)
	}

	nodeA := newReceiver("fork-node-a", dirA)
	nodeB := newReceiver("fork-node-b", dirB)

	type outcome struct {
		appended bool
		err      error
	}

	var resultA, resultB outcome
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		resultA.appended, resultA.err =
			nodeA.acceptBlock(blockA)
	}()

	go func() {
		defer wg.Done()
		resultB.appended, resultB.err =
			nodeB.acceptBlock(blockB)
	}()

	wg.Wait()

	if resultA.err != nil || !resultA.appended {
		t.Fatalf("node A acceptance: %+v", resultA)
	}
	if resultB.err != nil || !resultB.appended {
		t.Fatalf("node B acceptance: %+v", resultB)
	}

	if nodeA.Chain.Blocks[len(nodeA.Chain.Blocks)-1].Hash != blockA.Hash ||
		nodeB.Chain.Blocks[len(nodeB.Chain.Blocks)-1].Hash != blockB.Hash {
		t.Fatal("unexpected local chain tips")
	}

	for _, tc := range []struct {
		name      string
		server    *Server
		rival     blockchain.Block
		expected  string
		directory string
	}{
		{"A", nodeA, blockB, blockA.Hash, dirA},
		{"B", nodeB, blockA, blockB.Hash, dirB},
	} {
		t.Run(tc.name, func(t *testing.T) {
			before, err := json.Marshal(tc.server.Chain)
			if err != nil {
				t.Fatal(err)
			}

			appended, err := tc.server.acceptBlock(tc.rival)
			if err == nil || appended {
				t.Fatal("conflicting block unexpectedly accepted")
			}

			after, err := json.Marshal(tc.server.Chain)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(before, after) {
				t.Fatal("conflict changed local chain")
			}

			if !tc.server.Chain.ValidateChain(pos) {
				t.Fatal("local chain invalid")
			}

			record, err := tc.server.Chain.GetEscrowRecordV065(jobID)
			if err != nil || record.Status != escrow.Released {
				t.Fatalf("unexpected escrow: %+v %v", record, err)
			}

			saved, savedPoS, _, err := storage.Load(tc.directory)
			if err != nil {
				t.Fatal(err)
			}
			if !saved.ValidateChain(savedPoS) {
				t.Fatal("stored chain invalid")
			}

			last := saved.Blocks[len(saved.Blocks)-1]
			if last.Hash != tc.expected {
				t.Fatal("stored fork tip changed")
			}

			t.Logf(
				"valid local fork preserved: height=%d hash=%s",
				last.Height, last.Hash,
			)
		})
	}

	if nodeA.Chain.Blocks[len(nodeA.Chain.Blocks)-1].Hash ==
		nodeB.Chain.Blocks[len(nodeB.Chain.Blocks)-1].Hash {
		t.Fatal("expected nodes to remain on distinct branches")
	}

	t.Log("two individually valid histories remain divergent")
}
