package reserved

import (
	"strings"
	"testing"

	"prism/internal/consensus"
	"prism/internal/wallet"
)

func TestGovernanceStateAppliesAuthorityChange(
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

	const chainID = "prism-governance-state-test"

	change :=
		NewAuthorityChange(
			chainID,
			1,
			consensus.ReservedPoolTreasury,
			AuthorityChangeAdd,
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

	if err := state.ApplyAuthorityChange(
		change,
		chainID,
	); err != nil {
		t.Fatal(err)
	}

	authorized, err :=
		state.CurrentPolicy.IsAuthorized(
			consensus.ReservedPoolTreasury,
			target.Address,
		)

	if err != nil {
		t.Fatal(err)
	}

	if !authorized {
		t.Fatal(
			"new treasury authority was not activated",
		)
	}
}

func TestGovernanceStateRejectsAuthorityChangeReplay(
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

	const chainID = "prism-governance-state-test"

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

	err =
		state.Replay.ValidateAuthorityChangeNext(
			change,
		)

	if err == nil {
		t.Fatal(
			"expected authority change replay to fail",
		)
	}

	if !strings.Contains(
		err.Error(),
		"already used",
	) {
		t.Fatalf(
			"unexpected replay error: %v",
			err,
		)
	}
}

func TestGovernanceStateFailureDoesNotMutatePolicy(
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

	const chainID = "prism-governance-state-test"

	change :=
		NewAuthorityChange(
			chainID,
			1,
			consensus.ReservedPoolTreasury,
			AuthorityChangeAdd,
			target.Address,
		)

	// Only one approval for a 2-of-2 policy.
	if err := change.AddApproval(
		authorityA.Address,
		authorityA.PublicKeyHex(),
		authorityA.PrivateKey,
	); err != nil {
		t.Fatal(err)
	}

	if err := state.ApplyAuthorityChange(
		change,
		chainID,
	); err == nil {
		t.Fatal(
			"expected authority change below threshold to fail",
		)
	}

	authorized, err :=
		state.CurrentPolicy.IsAuthorized(
			consensus.ReservedPoolTreasury,
			target.Address,
		)

	if err != nil {
		t.Fatal(err)
	}

	if authorized {
		t.Fatal(
			"failed authority change mutated current policy",
		)
	}

	if err := state.Replay.ValidateAuthorityChangeNext(
		change,
	); err != nil {
		t.Fatalf(
			"failed authority change consumed replay state: %v",
			err,
		)
	}
}
