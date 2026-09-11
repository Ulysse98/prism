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

func timelockedGovernanceFixture(
	t *testing.T,
	activationHeight uint64,
) (
	*GovernanceState,
	AuthorityChange,
	*wallet.Wallet,
	string,
) {
	t.Helper()

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

	const chainID = "prism-governance-timelock-test"

	state, err :=
		NewGovernanceStateForChain(
			chainID,
			policy,
		)

	if err != nil {
		t.Fatal(err)
	}

	change :=
		NewTimelockedAuthorityChange(
			chainID,
			1,
			consensus.ReservedPoolTreasury,
			AuthorityChangeAdd,
			target.Address,
			activationHeight,
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

	return state,
		change,
		target,
		chainID
}

func TestGovernanceStateRejectsTimelockedChangeBeforeActivation(
	t *testing.T,
) {
	state, change, target, chainID :=
		timelockedGovernanceFixture(
			t,
			10,
		)

	err :=
		state.ApplyAuthorityChangeAtHeight(
			change,
			chainID,
			9,
		)

	if err == nil {
		t.Fatal(
			"expected timelocked authority change to fail before activation",
		)
	}

	if !strings.Contains(
		err.Error(),
		"timelock not reached",
	) {
		t.Fatalf(
			"unexpected timelock error: %v",
			err,
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
			"authority became active before timelock",
		)
	}
}

func TestGovernanceStateAcceptsTimelockedChangeAtActivation(
	t *testing.T,
) {
	state, change, target, chainID :=
		timelockedGovernanceFixture(
			t,
			10,
		)

	if err :=
		state.ApplyAuthorityChangeAtHeight(
			change,
			chainID,
			10,
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
			"authority was not activated at timelock boundary",
		)
	}
}

func TestGovernanceStateAcceptsTimelockedChangeAfterActivation(
	t *testing.T,
) {
	state, change, target, chainID :=
		timelockedGovernanceFixture(
			t,
			10,
		)

	if err :=
		state.ApplyAuthorityChangeAtHeight(
			change,
			chainID,
			11,
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
			"authority was not activated after timelock boundary",
		)
	}
}

func TestGovernanceStateTimelockFailureDoesNotConsumeReplayState(
	t *testing.T,
) {
	state, change, _, chainID :=
		timelockedGovernanceFixture(
			t,
			10,
		)

	err :=
		state.ApplyAuthorityChangeAtHeight(
			change,
			chainID,
			9,
		)

	if err == nil {
		t.Fatal(
			"expected authority change before activation to fail",
		)
	}

	key :=
		authorityChangeReplayKey{
			Pool: consensus.ReservedPoolTreasury,
		}

	if _, exists :=
		state.Replay.lastAuthorityChangeNonce[key]; exists {

		t.Fatal(
			"timelock failure consumed authority change nonce",
		)
	}

	if _, exists :=
		state.Replay.usedAuthorityChangeIDs[change.ID]; exists {

		t.Fatal(
			"timelock failure recorded authority change ID",
		)
	}

	// The exact same signed change must still be usable once
	// its activation height is reached.
	if err :=
		state.ApplyAuthorityChangeAtHeight(
			change,
			chainID,
			10,
		); err != nil {

		t.Fatalf(
			"authority change was not reusable at activation height: %v",
			err,
		)
	}
}
