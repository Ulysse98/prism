package reserved

import (
	"strings"
	"testing"

	"prism/internal/consensus"
	"prism/internal/wallet"
)

func authorityChangePolicyFixture(
	t *testing.T,
) (
	AuthorityPolicy,
	[]*wallet.Wallet,
	*wallet.Wallet,
) {
	t.Helper()

	authorities :=
		make(
			[]*wallet.Wallet,
			3,
		)

	for i := range authorities {
		current, err := wallet.New()

		if err != nil {
			t.Fatal(err)
		}

		authorities[i] = current
	}

	target, err := wallet.New()

	if err != nil {
		t.Fatal(err)
	}

	policy := AuthorityPolicy{
		Treasury: []string{
			authorities[0].Address,
			authorities[1].Address,
			authorities[2].Address,
		},
		TreasuryThreshold: 2,
	}

	return policy, authorities, target
}

func TestAuthorityPolicyValidatesThresholdAuthorityChange(
	t *testing.T,
) {
	policy, authorities, target :=
		authorityChangePolicyFixture(t)

	change :=
		NewAuthorityChange(
			"prism-governance-test",
			1,
			consensus.ReservedPoolTreasury,
			AuthorityChangeAdd,
			target.Address,
		)

	for _, authority := range authorities[:2] {

		if err :=
			change.AddApproval(
				authority.Address,
				authority.PublicKeyHex(),
				authority.PrivateKey,
			); err != nil {

			t.Fatal(err)
		}
	}

	if err :=
		policy.ValidateAuthorityChange(
			change,
			"prism-governance-test",
		); err != nil {

		t.Fatal(err)
	}
}

func TestAuthorityPolicyRejectsAuthorityChangeBelowThreshold(
	t *testing.T,
) {
	policy, authorities, target :=
		authorityChangePolicyFixture(t)

	change :=
		NewAuthorityChange(
			"prism-governance-test",
			1,
			consensus.ReservedPoolTreasury,
			AuthorityChangeAdd,
			target.Address,
		)

	if err :=
		change.AddApproval(
			authorities[0].Address,
			authorities[0].PublicKeyHex(),
			authorities[0].PrivateKey,
		); err != nil {

		t.Fatal(err)
	}

	err :=
		policy.ValidateAuthorityChange(
			change,
			"prism-governance-test",
		)

	if err == nil {
		t.Fatal(
			"expected authority change below threshold to fail",
		)
	}

	if !strings.Contains(
		err.Error(),
		"approval threshold not met",
	) {
		t.Fatalf(
			"unexpected threshold error: %v",
			err,
		)
	}
}

func TestAuthorityPolicyRejectsUnauthorizedAuthorityChangeApprover(
	t *testing.T,
) {
	policy, authorities, target :=
		authorityChangePolicyFixture(t)

	outsider, err := wallet.New()

	if err != nil {
		t.Fatal(err)
	}

	change :=
		NewAuthorityChange(
			"prism-governance-test",
			1,
			consensus.ReservedPoolTreasury,
			AuthorityChangeAdd,
			target.Address,
		)

	signers := []*wallet.Wallet{
		authorities[0],
		outsider,
	}

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

	err =
		policy.ValidateAuthorityChange(
			change,
			"prism-governance-test",
		)

	if err == nil {
		t.Fatal(
			"expected unauthorized authority change approver to fail",
		)
	}

	if !strings.Contains(
		err.Error(),
		"approver is not authorized",
	) {
		t.Fatalf(
			"unexpected unauthorized approver error: %v",
			err,
		)
	}
}

func TestAuthorityPolicyRejectsAuthorityChangeChainMismatch(
	t *testing.T,
) {
	policy, authorities, target :=
		authorityChangePolicyFixture(t)

	change :=
		NewAuthorityChange(
			"prism-governance-test",
			1,
			consensus.ReservedPoolTreasury,
			AuthorityChangeRemove,
			target.Address,
		)

	for _, authority := range authorities[:2] {

		if err :=
			change.AddApproval(
				authority.Address,
				authority.PublicKeyHex(),
				authority.PrivateKey,
			); err != nil {

			t.Fatal(err)
		}
	}

	err :=
		policy.ValidateAuthorityChange(
			change,
			"wrong-chain",
		)

	if err == nil {
		t.Fatal(
			"expected authority change chain mismatch to fail",
		)
	}

	if !strings.Contains(
		err.Error(),
		"chain ID mismatch",
	) {
		t.Fatalf(
			"unexpected chain mismatch error: %v",
			err,
		)
	}
}

func TestAuthorityPolicyRejectsDuplicateAuthorityChangeApprover(
	t *testing.T,
) {
	policy, authorities, target :=
		authorityChangePolicyFixture(t)

	change :=
		NewAuthorityChange(
			"prism-governance-test",
			1,
			consensus.ReservedPoolTreasury,
			AuthorityChangeAdd,
			target.Address,
		)

	for _, authority := range authorities[:2] {

		if err :=
			change.AddApproval(
				authority.Address,
				authority.PublicKeyHex(),
				authority.PrivateKey,
			); err != nil {

			t.Fatal(err)
		}
	}

	change.Approvals =
		append(
			change.Approvals,
			change.Approvals[0],
		)

	err :=
		policy.ValidateAuthorityChange(
			change,
			"prism-governance-test",
		)

	if err == nil {
		t.Fatal(
			"expected duplicate authority change approver to fail",
		)
	}

	if !strings.Contains(
		err.Error(),
		"duplicate reserved authority change approver",
	) {
		t.Fatalf(
			"unexpected duplicate approver error: %v",
			err,
		)
	}
}
