package reserved

import (
	"strings"
	"testing"

	"prism/internal/consensus"
	"prism/internal/wallet"
)

func signedAuthorityChangeForReplay(
	t *testing.T,
	policy AuthorityPolicy,
	signers []*wallet.Wallet,
	nonce uint64,
	pool consensus.ReservedPool,
	action AuthorityChangeAction,
	target string,
) AuthorityChange {
	t.Helper()

	change :=
		NewAuthorityChange(
			"prism-governance-replay-test",
			nonce,
			pool,
			action,
			target,
		)

	for _, signer := range signers {
		if err :=
			change.AddApproval(
				signer.Address,
				signer.PublicKeyHex(),
				signer.PrivateKey,
			); err != nil {

			t.Fatal(err)
		}
	}

	if err :=
		policy.ValidateAuthorityChange(
			change,
			"prism-governance-replay-test",
		); err != nil {

		t.Fatal(err)
	}

	return change
}

func TestAcceptAuthorityChangeRecordsReplayState(
	t *testing.T,
) {
	policy, authorities, target :=
		authorityChangePolicyFixture(t)

	change :=
		signedAuthorityChangeForReplay(
			t,
			policy,
			authorities[:2],
			1,
			consensus.ReservedPoolTreasury,
			AuthorityChangeAdd,
			target.Address,
		)

	state :=
		NewReplayState()

	updated, err :=
		state.AcceptAuthorityChange(
			change,
			policy,
			"prism-governance-replay-test",
		)

	if err != nil {
		t.Fatal(err)
	}

	authorized, err :=
		updated.IsAuthorized(
			consensus.ReservedPoolTreasury,
			target.Address,
		)

	if err != nil {
		t.Fatal(err)
	}

	if !authorized {
		t.Fatal(
			"accepted authority change was not applied",
		)
	}

	key :=
		authorityChangeReplayKey{
			Pool: consensus.ReservedPoolTreasury,
		}

	if state.lastAuthorityChangeNonce[key] != 1 {
		t.Fatalf(
			"expected treasury authority change nonce 1, got %d",
			state.lastAuthorityChangeNonce[key],
		)
	}

	if _, exists :=
		state.usedAuthorityChangeIDs[change.ID]; !exists {

		t.Fatal(
			"accepted authority change ID was not recorded",
		)
	}
}

func TestAcceptAuthorityChangeRejectsExactReplay(
	t *testing.T,
) {
	policy, authorities, target :=
		authorityChangePolicyFixture(t)

	change :=
		signedAuthorityChangeForReplay(
			t,
			policy,
			authorities[:2],
			1,
			consensus.ReservedPoolTreasury,
			AuthorityChangeAdd,
			target.Address,
		)

	state :=
		NewReplayState()

	updated, err :=
		state.AcceptAuthorityChange(
			change,
			policy,
			"prism-governance-replay-test",
		)

	if err != nil {
		t.Fatal(err)
	}

	_, err =
		state.AcceptAuthorityChange(
			change,
			updated,
			"prism-governance-replay-test",
		)

	if err == nil {
		t.Fatal(
			"expected exact authority change replay to fail",
		)
	}

	// Because the target is already configured after the first change,
	// ApplyAuthorityChange may reject the operation before replay logic.
	// ValidateAuthorityChangeNext must independently expose replay status.
	err =
		state.ValidateAuthorityChangeNext(
			change,
		)

	if err == nil ||
		!strings.Contains(
			err.Error(),
			"already used",
		) {

		t.Fatalf(
			"expected already-used replay error, got: %v",
			err,
		)
	}
}

func TestAuthorityChangeReplayRejectsLowerNonce(
	t *testing.T,
) {
	policy, authorities, firstTarget :=
		authorityChangePolicyFixture(t)

	state :=
		NewReplayState()

	first :=
		signedAuthorityChangeForReplay(
			t,
			policy,
			authorities[:2],
			2,
			consensus.ReservedPoolTreasury,
			AuthorityChangeAdd,
			firstTarget.Address,
		)

	updated, err :=
		state.AcceptAuthorityChange(
			first,
			policy,
			"prism-governance-replay-test",
		)

	if err != nil {
		t.Fatal(err)
	}

	secondTarget, err :=
		wallet.New()

	if err != nil {
		t.Fatal(err)
	}

	second :=
		NewAuthorityChange(
			"prism-governance-replay-test",
			1,
			consensus.ReservedPoolTreasury,
			AuthorityChangeAdd,
			secondTarget.Address,
		)

	for _, signer := range authorities[:2] {

		if err :=
			second.AddApproval(
				signer.Address,
				signer.PublicKeyHex(),
				signer.PrivateKey,
			); err != nil {

			t.Fatal(err)
		}
	}

	if err :=
		updated.ValidateAuthorityChange(
			second,
			"prism-governance-replay-test",
		); err != nil {

		t.Fatal(err)
	}

	err =
		state.ValidateAuthorityChangeNext(
			second,
		)

	if err == nil {
		t.Fatal(
			"expected lower authority change nonce to fail",
		)
	}

	if !strings.Contains(
		err.Error(),
		"nonce is not increasing",
	) {
		t.Fatalf(
			"unexpected lower nonce error: %v",
			err,
		)
	}
}

func TestAuthorityChangeReplayNonceIsPerPool(
	t *testing.T,
) {
	treasuryA, err := wallet.New()
	if err != nil {
		t.Fatal(err)
	}

	treasuryB, err := wallet.New()
	if err != nil {
		t.Fatal(err)
	}

	ecosystemA, err := wallet.New()
	if err != nil {
		t.Fatal(err)
	}

	ecosystemB, err := wallet.New()
	if err != nil {
		t.Fatal(err)
	}

	treasuryTarget, err := wallet.New()
	if err != nil {
		t.Fatal(err)
	}

	ecosystemTarget, err := wallet.New()
	if err != nil {
		t.Fatal(err)
	}

	policy := AuthorityPolicy{
		Treasury: []string{
			treasuryA.Address,
			treasuryB.Address,
		},
		TreasuryThreshold: 2,

		Ecosystem: []string{
			ecosystemA.Address,
			ecosystemB.Address,
		},
		EcosystemThreshold: 2,
	}

	state :=
		NewReplayState()

	treasuryChange :=
		signedAuthorityChangeForReplay(
			t,
			policy,
			[]*wallet.Wallet{
				treasuryA,
				treasuryB,
			},
			7,
			consensus.ReservedPoolTreasury,
			AuthorityChangeAdd,
			treasuryTarget.Address,
		)

	updated, err :=
		state.AcceptAuthorityChange(
			treasuryChange,
			policy,
			"prism-governance-replay-test",
		)

	if err != nil {
		t.Fatal(err)
	}

	ecosystemChange :=
		signedAuthorityChangeForReplay(
			t,
			updated,
			[]*wallet.Wallet{
				ecosystemA,
				ecosystemB,
			},
			1,
			consensus.ReservedPoolEcosystem,
			AuthorityChangeAdd,
			ecosystemTarget.Address,
		)

	if _, err :=
		state.AcceptAuthorityChange(
			ecosystemChange,
			updated,
			"prism-governance-replay-test",
		); err != nil {

		t.Fatal(
			"authority change nonce should be independent per pool:",
			err,
		)
	}
}

func TestRejectedAuthorityChangeDoesNotAdvanceReplayState(
	t *testing.T,
) {
	policy, authorities, target :=
		authorityChangePolicyFixture(t)

	change :=
		NewAuthorityChange(
			"prism-governance-replay-test",
			5,
			consensus.ReservedPoolTreasury,
			AuthorityChangeAdd,
			target.Address,
		)

	// Only one signature against a 2-of-3 policy.
	if err :=
		change.AddApproval(
			authorities[0].Address,
			authorities[0].PublicKeyHex(),
			authorities[0].PrivateKey,
		); err != nil {

		t.Fatal(err)
	}

	state :=
		NewReplayState()

	if _, err :=
		state.AcceptAuthorityChange(
			change,
			policy,
			"prism-governance-replay-test",
		); err == nil {

		t.Fatal(
			"expected under-threshold authority change to fail",
		)
	}

	key :=
		authorityChangeReplayKey{
			Pool: consensus.ReservedPoolTreasury,
		}

	if _, exists :=
		state.lastAuthorityChangeNonce[key]; exists {

		t.Fatal(
			"rejected authority change advanced nonce",
		)
	}

	if _, exists :=
		state.usedAuthorityChangeIDs[change.ID]; exists {

		t.Fatal(
			"rejected authority change recorded its ID",
		)
	}
}
