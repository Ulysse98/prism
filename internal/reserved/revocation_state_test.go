package reserved

import (
	"testing"

	"prism/internal/consensus"
	"prism/internal/wallet"
)

func approveTestRevocation(
	t *testing.T,
	revocation *Revocation,
	authority *wallet.Wallet,
) {
	t.Helper()

	if err :=
		revocation.AddApproval(
			authority.Address,
			authority.PublicKeyHex(),
			authority.PrivateKey,
		); err != nil {

		t.Fatal(err)
	}
}

func newThresholdRevocationTest(
	t *testing.T,
	threshold uint32,
) (
	Grant,
	Revocation,
	AuthorityPolicy,
	[]*wallet.Wallet,
) {
	t.Helper()

	grant, policy, authorities :=
		newThresholdGrantTest(
			t,
			threshold,
		)

	revocation :=
		NewRevocation(
			testChainID,
			grant.Pool,
			grant.ID,
		)

	return grant,
		revocation,
		policy,
		authorities
}

func TestAuthorityPolicyAcceptsTwoOfThreeRevocation(
	t *testing.T,
) {
	_, revocation, policy, authorities :=
		newThresholdRevocationTest(
			t,
			2,
		)

	approveTestRevocation(
		t,
		&revocation,
		authorities[0],
	)

	approveTestRevocation(
		t,
		&revocation,
		authorities[1],
	)

	if err :=
		policy.ValidateRevocation(
			revocation,
			testChainID,
		); err != nil {

		t.Fatal(err)
	}
}

func TestAuthorityPolicyRejectsRevocationBelowThreshold(
	t *testing.T,
) {
	_, revocation, policy, authorities :=
		newThresholdRevocationTest(
			t,
			2,
		)

	approveTestRevocation(
		t,
		&revocation,
		authorities[0],
	)

	if err :=
		policy.ValidateRevocation(
			revocation,
			testChainID,
		); err == nil {

		t.Fatal(
			"expected revocation below threshold to fail",
		)
	}
}

func TestReplayStateRejectsRevokedGrant(
	t *testing.T,
) {
	grant, revocation, policy, authorities :=
		newThresholdRevocationTest(
			t,
			2,
		)

	approveTestRevocation(
		t,
		&revocation,
		authorities[0],
	)

	approveTestRevocation(
		t,
		&revocation,
		authorities[1],
	)

	replay :=
		NewReplayState()

	if err :=
		replay.AcceptRevocation(
			revocation,
			policy,
			testChainID,
		); err != nil {

		t.Fatal(err)
	}

	if err :=
		replay.ValidateGrantNext(
			grant,
		); err == nil {

		t.Fatal(
			"expected revoked grant to be rejected",
		)
	}
}

func TestWrongPoolRevocationDoesNotRevokeGrant(
	t *testing.T,
) {
	grant, _, _, authorities :=
		newThresholdRevocationTest(
			t,
			2,
		)

	addresses := []string{
		authorities[0].Address,
		authorities[1].Address,
		authorities[2].Address,
	}

	policy := AuthorityPolicy{
		Ecosystem:          addresses,
		EcosystemThreshold: 2,
	}

	revocation :=
		NewRevocation(
			testChainID,
			consensus.ReservedPoolEcosystem,
			grant.ID,
		)

	approveTestRevocation(
		t,
		&revocation,
		authorities[0],
	)

	approveTestRevocation(
		t,
		&revocation,
		authorities[1],
	)

	replay :=
		NewReplayState()

	if err :=
		replay.AcceptRevocation(
			revocation,
			policy,
			testChainID,
		); err != nil {

		t.Fatal(err)
	}

	if err :=
		replay.ValidateGrantNext(
			grant,
		); err != nil {

		t.Fatalf(
			"wrong-pool revocation must not block grant: %v",
			err,
		)
	}
}

func TestCannotRevokeExecutedGrant(
	t *testing.T,
) {
	grant, revocation, policy, authorities :=
		newThresholdRevocationTest(
			t,
			2,
		)

	replay :=
		NewReplayState()

	if err :=
		replay.AcceptGrant(
			grant,
		); err != nil {

		t.Fatal(err)
	}

	approveTestRevocation(
		t,
		&revocation,
		authorities[0],
	)

	approveTestRevocation(
		t,
		&revocation,
		authorities[1],
	)

	if err :=
		replay.AcceptRevocation(
			revocation,
			policy,
			testChainID,
		); err == nil {

		t.Fatal(
			"expected executed grant revocation to fail",
		)
	}
}

func TestAccountingRevocationDoesNotConsumeBudget(
	t *testing.T,
) {
	_, revocation, policy, authorities :=
		newThresholdRevocationTest(
			t,
			2,
		)

	approveTestRevocation(
		t,
		&revocation,
		authorities[0],
	)

	approveTestRevocation(
		t,
		&revocation,
		authorities[1],
	)

	state :=
		NewAccountingState(0)

	if err :=
		state.AcceptRevocation(
			revocation,
			policy,
			testChainID,
		); err != nil {

		t.Fatal(err)
	}

	if state.Usage.Ecosystem != 0 ||
		state.Usage.Treasury != 0 ||
		state.Usage.Team != 0 ||
		state.Usage.Liquidity != 0 {

		t.Fatal(
			"revocation consumed reserved budget",
		)
	}
}
