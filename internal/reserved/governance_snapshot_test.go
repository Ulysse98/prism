package reserved

import (
	"encoding/json"
	"strings"
	"testing"

	"prism/internal/consensus"
	"prism/internal/wallet"
)

func TestGovernanceSnapshotRoundTripPreservesPolicyAndReplay(
	t *testing.T,
) {
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

	policy := AuthorityPolicy{
		Treasury: []string{
			authorityA.Address,
			authorityB.Address,
		},
		TreasuryThreshold: 2,
	}

	state, err :=
		NewGovernanceState(policy)

	if err != nil {
		t.Fatal(err)
	}

	const chainID = "prism-governance-snapshot-test"

	change :=
		NewAuthorityChange(
			chainID,
			1,
			consensus.ReservedPoolTreasury,
			AuthorityChangeAdd,
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

	snapshot, err := state.Snapshot()
	if err != nil {
		t.Fatal(err)
	}

	data, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatal(err)
	}

	var decoded GovernanceSnapshot

	if err := json.Unmarshal(
		data,
		&decoded,
	); err != nil {
		t.Fatal(err)
	}

	restored, err :=
		GovernanceStateFromSnapshot(
			decoded,
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
			"restored governance policy lost new authority",
		)
	}

	err =
		restored.Replay.ValidateAuthorityChangeNext(
			change,
		)

	if err == nil {
		t.Fatal(
			"restored governance replay state accepted old authority change",
		)
	}

	if !strings.Contains(
		err.Error(),
		"already used",
	) {
		t.Fatalf(
			"unexpected replay error after governance restore: %v",
			err,
		)
	}
}

func TestGovernanceSnapshotRejectsInvalidPolicy(
	t *testing.T,
) {
	snapshot := GovernanceSnapshot{
		CurrentPolicy: AuthorityPolicy{
			Treasury: []string{
				"authority-a",
			},
			TreasuryThreshold: 2,
		},
	}

	_, err :=
		GovernanceStateFromSnapshot(
			snapshot,
		)

	if err == nil {
		t.Fatal(
			"expected invalid stored governance policy to fail",
		)
	}

	if !strings.Contains(
		err.Error(),
		"invalid stored reserved authority policy",
	) {
		t.Fatalf(
			"unexpected invalid governance policy error: %v",
			err,
		)
	}
}
