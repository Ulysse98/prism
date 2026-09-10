package reserved

import (
	"reflect"
	"sort"
	"strings"
	"testing"

	"prism/internal/consensus"
	"prism/internal/wallet"
)

func signAuthorityChangeForApply(
	t *testing.T,
	change *AuthorityChange,
	signers []*wallet.Wallet,
) {
	t.Helper()

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
}

func TestApplyAuthorityChangeAddsAuthority(
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

	signAuthorityChangeForApply(
		t,
		&change,
		authorities[:2],
	)

	updated, err :=
		policy.ApplyAuthorityChange(
			change,
			"prism-governance-test",
		)

	if err != nil {
		t.Fatal(err)
	}

	if len(policy.Treasury) != 3 {
		t.Fatal(
			"original policy was mutated",
		)
	}

	if len(updated.Treasury) != 4 {
		t.Fatalf(
			"expected four authorities, got %d",
			len(updated.Treasury),
		)
	}

	expected :=
		append(
			[]string(nil),
			policy.Treasury...,
		)

	expected =
		append(
			expected,
			target.Address,
		)

	sort.Strings(expected)

	if !reflect.DeepEqual(
		updated.Treasury,
		expected,
	) {
		t.Fatalf(
			"authority list is not canonical: got=%v expected=%v",
			updated.Treasury,
			expected,
		)
	}
}

func TestApplyAuthorityChangeRemovesAuthority(
	t *testing.T,
) {
	policy, authorities, _ :=
		authorityChangePolicyFixture(t)

	target :=
		authorities[2]

	change :=
		NewAuthorityChange(
			"prism-governance-test",
			1,
			consensus.ReservedPoolTreasury,
			AuthorityChangeRemove,
			target.Address,
		)

	signAuthorityChangeForApply(
		t,
		&change,
		authorities[:2],
	)

	updated, err :=
		policy.ApplyAuthorityChange(
			change,
			"prism-governance-test",
		)

	if err != nil {
		t.Fatal(err)
	}

	if len(updated.Treasury) != 2 {
		t.Fatalf(
			"expected two authorities, got %d",
			len(updated.Treasury),
		)
	}

	authorized, err :=
		updated.IsAuthorized(
			consensus.ReservedPoolTreasury,
			target.Address,
		)

	if err != nil {
		t.Fatal(err)
	}

	if authorized {
		t.Fatal(
			"removed authority is still authorized",
		)
	}

	threshold, err :=
		updated.EffectiveThreshold(
			consensus.ReservedPoolTreasury,
		)

	if err != nil {
		t.Fatal(err)
	}

	if threshold != 2 {
		t.Fatalf(
			"expected threshold 2, got %d",
			threshold,
		)
	}
}

func TestApplyAuthorityChangeRejectsExistingAdd(
	t *testing.T,
) {
	policy, authorities, _ :=
		authorityChangePolicyFixture(t)

	change :=
		NewAuthorityChange(
			"prism-governance-test",
			1,
			consensus.ReservedPoolTreasury,
			AuthorityChangeAdd,
			authorities[2].Address,
		)

	signAuthorityChangeForApply(
		t,
		&change,
		authorities[:2],
	)

	_, err :=
		policy.ApplyAuthorityChange(
			change,
			"prism-governance-test",
		)

	if err == nil {
		t.Fatal(
			"expected existing authority add to fail",
		)
	}

	if !strings.Contains(
		err.Error(),
		"already configured",
	) {
		t.Fatalf(
			"unexpected existing authority error: %v",
			err,
		)
	}
}

func TestApplyAuthorityChangeRejectsMissingRemove(
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

	signAuthorityChangeForApply(
		t,
		&change,
		authorities[:2],
	)

	_, err :=
		policy.ApplyAuthorityChange(
			change,
			"prism-governance-test",
		)

	if err == nil {
		t.Fatal(
			"expected missing authority removal to fail",
		)
	}

	if !strings.Contains(
		err.Error(),
		"not configured",
	) {
		t.Fatalf(
			"unexpected missing authority error: %v",
			err,
		)
	}
}

func TestApplyAuthorityChangeRejectsThresholdBreak(
	t *testing.T,
) {
	first, err := wallet.New()
	if err != nil {
		t.Fatal(err)
	}

	second, err := wallet.New()
	if err != nil {
		t.Fatal(err)
	}

	policy := AuthorityPolicy{
		Treasury: []string{
			first.Address,
			second.Address,
		},
		TreasuryThreshold: 2,
	}

	change :=
		NewAuthorityChange(
			"prism-governance-test",
			1,
			consensus.ReservedPoolTreasury,
			AuthorityChangeRemove,
			second.Address,
		)

	signAuthorityChangeForApply(
		t,
		&change,
		[]*wallet.Wallet{
			first,
			second,
		},
	)

	_, err =
		policy.ApplyAuthorityChange(
			change,
			"prism-governance-test",
		)

	if err == nil {
		t.Fatal(
			"expected threshold-breaking removal to fail",
		)
	}

	if !strings.Contains(
		err.Error(),
		"threshold exceeds authority count",
	) {
		t.Fatalf(
			"unexpected threshold break error: %v",
			err,
		)
	}
}

func TestApplyAuthorityChangeRejectsLastAuthorityRemoval(
	t *testing.T,
) {
	authority, err := wallet.New()
	if err != nil {
		t.Fatal(err)
	}

	policy := AuthorityPolicy{
		Treasury: []string{
			authority.Address,
		},
	}

	change :=
		NewAuthorityChange(
			"prism-governance-test",
			1,
			consensus.ReservedPoolTreasury,
			AuthorityChangeRemove,
			authority.Address,
		)

	signAuthorityChangeForApply(
		t,
		&change,
		[]*wallet.Wallet{
			authority,
		},
	)

	_, err =
		policy.ApplyAuthorityChange(
			change,
			"prism-governance-test",
		)

	if err == nil {
		t.Fatal(
			"expected last authority removal to fail",
		)
	}

	if !strings.Contains(
		err.Error(),
		"cannot remove last reserved authority",
	) {
		t.Fatalf(
			"unexpected last authority error: %v",
			err,
		)
	}
}
