package storage

import (
	"strings"
	"testing"

	"prism/internal/consensus"
	"prism/internal/reserved"
	"prism/internal/wallet"
)

func TestGovernanceStateDiskRoundTripPreservesPolicyAndReplay(
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

	policy := reserved.AuthorityPolicy{
		Treasury: []string{
			authorityA.Address,
			authorityB.Address,
		},
		TreasuryThreshold: 2,
	}

	state, err :=
		reserved.NewGovernanceState(
			policy,
		)

	if err != nil {
		t.Fatal(err)
	}

	const chainID = "prism-governance-disk-test"

	change :=
		reserved.NewAuthorityChange(
			chainID,
			1,
			consensus.ReservedPoolTreasury,
			reserved.AuthorityChangeAdd,
			target.Address,
		)

	for _, signer := range []*wallet.Wallet{
		authorityA,
		authorityB,
	} {
		if err := change.AddApproval(
			signer.Address,
			signer.PublicKeyHex(),
			signer.PrivateKey,
		); err != nil {
			t.Fatal(err)
		}
	}

	if err := state.ApplyAuthorityChange(
		change,
		chainID,
	); err != nil {
		t.Fatal(err)
	}

	if err := SaveGovernanceState(
		dataDir,
		state,
	); err != nil {
		t.Fatal(err)
	}

	if !GovernanceStateExists(
		dataDir,
	) {
		t.Fatal(
			"expected governance state file to exist",
		)
	}

	// Simulate a full process restart.
	state = nil

	restored, err :=
		LoadGovernanceState(
			dataDir,
		)

	if err != nil {
		t.Fatal(err)
	}

	authorized, err :=
		restored.CurrentPolicy.IsAuthorized(
			consensus.ReservedPoolTreasury,
			target.Address,
		)

	if err != nil {
		t.Fatal(err)
	}

	if !authorized {
		t.Fatal(
			"restored governance state lost new authority",
		)
	}

	err =
		restored.Replay.ValidateAuthorityChangeNext(
			change,
		)

	if err == nil {
		t.Fatal(
			"restored governance state accepted replayed authority change",
		)
	}

	if !strings.Contains(
		err.Error(),
		"already used",
	) {
		t.Fatalf(
			"unexpected replay error after restart: %v",
			err,
		)
	}
}

func TestLoadGovernanceStateMissingFile(
	t *testing.T,
) {
	dataDir := t.TempDir()

	if GovernanceStateExists(dataDir) {
		t.Fatal(
			"governance state should not exist",
		)
	}

	_, err :=
		LoadGovernanceState(
			dataDir,
		)

	if err == nil {
		t.Fatal(
			"expected missing governance state to fail",
		)
	}

	if !strings.Contains(
		err.Error(),
		"cannot load reserved governance state",
	) {
		t.Fatalf(
			"unexpected missing governance state error: %v",
			err,
		)
	}
}
