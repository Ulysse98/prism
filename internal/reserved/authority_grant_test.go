package reserved

import (
	"testing"

	"prism/internal/consensus"
	"prism/internal/wallet"
)

func newThresholdGrantTest(
	t *testing.T,
	threshold uint32,
) (
	Grant,
	AuthorityPolicy,
	[]*wallet.Wallet,
) {
	t.Helper()

	recipient, err := wallet.New()
	if err != nil {
		t.Fatal(err)
	}

	authorities := make(
		[]*wallet.Wallet,
		3,
	)

	addresses := make(
		[]string,
		3,
	)

	for i := range authorities {
		authorities[i], err =
			wallet.New()

		if err != nil {
			t.Fatal(err)
		}

		addresses[i] =
			authorities[i].Address
	}

	grant :=
		NewGrant(
			testChainID,
			1,
			consensus.ReservedPoolTreasury,
			recipient.Address,
			100,
		)

	policy := AuthorityPolicy{
		Treasury:          addresses,
		TreasuryThreshold: threshold,
	}

	return grant,
		policy,
		authorities
}

func approveThresholdGrant(
	t *testing.T,
	grant *Grant,
	authorizer *wallet.Wallet,
) {
	t.Helper()

	if err :=
		grant.AddApproval(
			authorizer.Address,
			authorizer.PublicKeyHex(),
			authorizer.PrivateKey,
		); err != nil {

		t.Fatal(err)
	}
}

func TestAuthorityPolicyAcceptsTwoOfThreeGrant(
	t *testing.T,
) {
	grant, policy, authorities :=
		newThresholdGrantTest(
			t,
			2,
		)

	approveThresholdGrant(
		t,
		&grant,
		authorities[0],
	)

	approveThresholdGrant(
		t,
		&grant,
		authorities[1],
	)

	if err :=
		policy.ValidateGrant(
			grant,
			testChainID,
		); err != nil {

		t.Fatal(err)
	}
}

func TestAuthorityPolicyRejectsGrantBelowThreshold(
	t *testing.T,
) {
	grant, policy, authorities :=
		newThresholdGrantTest(
			t,
			2,
		)

	approveThresholdGrant(
		t,
		&grant,
		authorities[0],
	)

	if err :=
		policy.ValidateGrant(
			grant,
			testChainID,
		); err == nil {

		t.Fatal(
			"expected one-of-three grant to fail two-of-three threshold",
		)
	}
}

func TestAuthorityPolicyRejectsUnauthorizedGrantApproval(
	t *testing.T,
) {
	grant, policy, authorities :=
		newThresholdGrantTest(
			t,
			2,
		)

	approveThresholdGrant(
		t,
		&grant,
		authorities[0],
	)

	outsider, err := wallet.New()
	if err != nil {
		t.Fatal(err)
	}

	approveThresholdGrant(
		t,
		&grant,
		outsider,
	)

	if err :=
		policy.ValidateGrant(
			grant,
			testChainID,
		); err == nil {

		t.Fatal(
			"expected unauthorized grant approver to fail",
		)
	}
}

func TestAuthorityPolicyRejectsDuplicateGrantApprover(
	t *testing.T,
) {
	grant, policy, authorities :=
		newThresholdGrantTest(
			t,
			2,
		)

	approveThresholdGrant(
		t,
		&grant,
		authorities[0],
	)

	grant.Approvals =
		append(
			grant.Approvals,
			grant.Approvals[0],
		)

	if err :=
		policy.ValidateGrant(
			grant,
			testChainID,
		); err == nil {

		t.Fatal(
			"expected duplicate grant approver to fail",
		)
	}
}

func TestAuthorityPolicyLegacyThresholdDefaultsToOne(
	t *testing.T,
) {
	grant, policy, authorities :=
		newThresholdGrantTest(
			t,
			0,
		)

	approveThresholdGrant(
		t,
		&grant,
		authorities[0],
	)

	if err :=
		policy.ValidateGrant(
			grant,
			testChainID,
		); err != nil {

		t.Fatal(err)
	}
}

func TestAuthorityPolicyRejectsImpossibleThreshold(
	t *testing.T,
) {
	_, policy, _ :=
		newThresholdGrantTest(
			t,
			4,
		)

	if err := policy.Validate(); err == nil {
		t.Fatal(
			"expected threshold above authority count to fail",
		)
	}
}

func TestAuthorityPolicyRejectsGrantForWrongChain(
	t *testing.T,
) {
	grant, policy, authorities :=
		newThresholdGrantTest(
			t,
			2,
		)

	approveThresholdGrant(
		t,
		&grant,
		authorities[0],
	)

	approveThresholdGrant(
		t,
		&grant,
		authorities[1],
	)

	if err :=
		policy.ValidateGrant(
			grant,
			"other-chain",
		); err == nil {

		t.Fatal(
			"expected wrong-chain grant to fail",
		)
	}
}
