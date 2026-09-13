package storage

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"prism/internal/consensus"
	"prism/internal/reserved"
	"prism/internal/wallet"
)

func TestReplayStateDiskRoundTripRejectsAuthorityChangeReplay(
	t *testing.T,
) {
	dataDir := t.TempDir()

	authorityA, err := wallet.New()
	if err != nil {
		t.Fatal(err)
	}

	authorityB, err := wallet.New()
	if err != nil {
		t.Fatal(err)
	}

	target, err := wallet.New()
	if err != nil {
		t.Fatal(err)
	}

	const chainID = "prism-storage-replay-test"

	policy := reserved.AuthorityPolicy{
		Treasury: []string{
			authorityA.Address,
			authorityB.Address,
		},
		TreasuryThreshold: 2,
	}

	change :=
		reserved.NewAuthorityChange(
			chainID,
			5,
			consensus.ReservedPoolTreasury,
			reserved.AuthorityChangeAdd,
			target.Address,
		)

	if err := change.AddApproval(
		authorityA.Address,
		authorityA.PublicKeyHex(),
		authorityA.PrivateKey,
	); err != nil {
		t.Fatal(err)
	}

	if err := change.AddApproval(
		authorityB.Address,
		authorityB.PublicKeyHex(),
		authorityB.PrivateKey,
	); err != nil {
		t.Fatal(err)
	}

	state := reserved.NewReplayState()

	_, err =
		state.AcceptAuthorityChange(
			change,
			policy,
			chainID,
		)

	if err != nil {
		t.Fatal(err)
	}

	if err := SaveReplayState(
		dataDir,
		state,
	); err != nil {
		t.Fatal(err)
	}

	if !ReplayStateExists(dataDir) {
		t.Fatal(
			"expected persisted replay state to exist",
		)
	}

	// Simulate process restart by discarding the
	// original in-memory state and loading from disk.
	restored, err :=
		LoadReplayState(
			dataDir,
		)

	if err != nil {
		t.Fatal(err)
	}

	err =
		restored.ValidateAuthorityChangeNext(
			change,
		)

	if err == nil {
		t.Fatal(
			"expected persisted replay state to reject authority change replay",
		)
	}

	if !strings.Contains(
		err.Error(),
		"already used",
	) {
		t.Fatalf(
			"expected already-used replay error after disk restore, got: %v",
			err,
		)
	}
}

func TestLoadReplayStateMissingFile(
	t *testing.T,
) {
	dataDir := t.TempDir()

	if ReplayStateExists(dataDir) {
		t.Fatal(
			"replay state should not exist",
		)
	}

	_, err :=
		LoadReplayState(
			dataDir,
		)

	if err == nil {
		t.Fatal(
			"expected missing replay state file to fail",
		)
	}

	if !strings.Contains(
		err.Error(),
		"cannot load reserved replay state",
	) {
		t.Fatalf(
			"unexpected missing file error: %v",
			err,
		)
	}
}

func TestLoadReplayStateRejectsCorruptState(
	t *testing.T,
) {
	dataDir := t.TempDir()

	path := filepath.Join(
		dataDir,
		replayStateFilename,
	)

	data := []byte(`{
		"authority_change_nonces": [
			{
				"pool": "not-a-real-pool",
				"nonce": 1
			}
		]
	}`)

	if err := os.WriteFile(
		path,
		data,
		0600,
	); err != nil {
		t.Fatal(err)
	}

	_, err :=
		LoadReplayState(
			dataDir,
		)

	if err == nil {
		t.Fatal(
			"expected corrupt replay state to fail",
		)
	}

	if !strings.Contains(
		err.Error(),
		"invalid stored reserved pool",
	) {
		t.Fatalf(
			"unexpected corrupt state error: %v",
			err,
		)
	}
}
