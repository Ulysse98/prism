package p2p

import (
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"prism/internal/blockchain"
	"prism/internal/escrow"
	"prism/internal/storage"
	"prism/internal/transaction"
	"prism/internal/wallet"
)

func TestEscrowV065P2PTCPRejectsLongerDivergentChain(t *testing.T) {
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

	if blockA.Height != 3 ||
		blockB.Height != 3 ||
		blockA.Hash == blockB.Hash {
		t.Fatal("expected two different RELEASE blocks at height 3")
	}

	// Extend only B with a valid, signed transaction at height 4.
	tx := transaction.New(
		alice.Address,
		bob.Address,
		1,
		0,
		alice.PublicKeyHex(),
	)
	if err := tx.Sign(alice.PrivateKey); err != nil {
		t.Fatal(err)
	}

	extra, err := branchB.AddBlock(
		[]transaction.Transaction{tx},
		nil,
		alice.Address,
		pos,
	)
	if err != nil {
		t.Fatalf("cannot extend branch B: %v", err)
	}
	if extra.Height != 4 {
		t.Fatal("expected B to reach height 4")
	}

	for name, chain := range map[string]*blockchain.Blockchain{
		"A": branchA,
		"B": branchB,
	} {
		if !chain.ValidateChain(pos) {
			t.Fatalf("invalid branch %s", name)
		}
		record, err := chain.GetEscrowRecordV065(jobID)
		if err != nil || record.Status != escrow.Released {
			t.Fatalf("invalid escrow state on %s: %+v %v",
				name, record, err)
		}
	}

	dirA := t.TempDir()
	dirB := t.TempDir()

	wallets := map[string]*wallet.Wallet{
		"Alice": alice,
		"Bob":   bob,
	}

	if err := storage.Save(dirA, branchA, pos, wallets); err != nil {
		t.Fatal(err)
	}
	if err := storage.Save(dirB, branchB, pos, wallets); err != nil {
		t.Fatal(err)
	}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()

	nodeA := NewServer(
		"longer-fork-node-a",
		"127.0.0.1:0",
		dirA, branchA, pos, wallets,
	)
	nodeB := NewServer(
		"longer-fork-node-b",
		listener.Addr().String(),
		dirB, branchB, pos, wallets,
	)

	type snapshot struct {
		Memory      []byte
		Disk        []byte
		Hash        string
		Height      uint64
		AliceFunds  uint64
		BobBalance  uint64
		TotalSupply uint64
	}

	capture := func(node *Server, dir string) snapshot {
		t.Helper()

		memory, err := json.Marshal(node.Chain)
		if err != nil {
			t.Fatal(err)
		}
		disk, err := os.ReadFile(filepath.Join(dir, "chain.json"))
		if err != nil {
			t.Fatal(err)
		}
		aliceFunds, err := node.Chain.AvailableBalanceOf(alice.Address)
		if err != nil {
			t.Fatal(err)
		}
		bobBalance, err := node.Chain.BalanceOf(bob.Address)
		if err != nil {
			t.Fatal(err)
		}
		supply, err := node.Chain.TotalSupply()
		if err != nil {
			t.Fatal(err)
		}

		tip := node.Chain.Blocks[len(node.Chain.Blocks)-1]

		return snapshot{
			Memory:      memory,
			Disk:        disk,
			Hash:        tip.Hash,
			Height:      tip.Height,
			AliceFunds:  aliceFunds,
			BobBalance:  bobBalance,
			TotalSupply: supply,
		}
	}

	beforeA := capture(nodeA, dirA)
	beforeB := capture(nodeB, dirB)

	if beforeA.Height != 3 || beforeB.Height != 4 {
		t.Fatal("wrong initial heights")
	}
	if beforeB.BobBalance != beforeA.BobBalance+1 {
		t.Fatal("expected extra transaction on longer fork")
	}

	done := make(chan error, 1)

	go func() {
		conn, err := listener.Accept()
		if err != nil {
			done <- err
			return
		}
		nodeB.handleIncoming(conn)
		done <- nil
	}()

	t.Log("A at height 3 connects to divergent B at height 4")

	connectErr := nodeA.Connect(listener.Addr().String())

	if connectErr == nil ||
		!strings.Contains(
			connectErr.Error(),
			"remote chain does not extend local history",
		) {
		t.Fatalf(
			"expected prefix-validation rejection, got %v",
			connectErr,
		)
	}

	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(12 * time.Second):
		t.Fatal("TCP receiver did not finish")
	}

	t.Logf("expected sync rejection: %v", connectErr)

	if !reflect.DeepEqual(beforeA, capture(nodeA, dirA)) {
		t.Fatal("A changed after rejecting longer fork")
	}
	if !reflect.DeepEqual(beforeB, capture(nodeB, dirB)) {
		t.Fatal("B changed during rejected synchronization")
	}

	for _, tc := range []struct {
		name string
		dir  string
		want snapshot
	}{
		{"A", dirA, beforeA},
		{"B", dirB, beforeB},
	} {
		restored, restoredPoS, _, err := storage.Load(tc.dir)
		if err != nil {
			t.Fatal(err)
		}
		if !restored.ValidateChain(restoredPoS) {
			t.Fatalf("%s: invalid persisted chain", tc.name)
		}

		tip := restored.Blocks[len(restored.Blocks)-1]
		if tip.Height != tc.want.Height ||
			tip.Hash != tc.want.Hash {
			t.Fatalf("%s: persisted fork replaced", tc.name)
		}

		t.Logf(
			"%s preserved: height=%d hash=%s",
			tc.name, tip.Height, tip.Hash,
		)
	}

	t.Log("longer divergent history rejected without chain or balance changes")
}
