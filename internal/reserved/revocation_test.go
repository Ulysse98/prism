package reserved

import (
	"testing"

	"prism/internal/consensus"
	"prism/internal/wallet"
)

func newTestRevocation(
	t *testing.T,
) Revocation {
	t.Helper()

	grant := newTestGrant(t)

	return NewRevocation(
		testChainID,
		consensus.ReservedPoolTreasury,
		grant.ID,
	)
}

func TestRevocationIDDoesNotDependOnApprovals(
	t *testing.T,
) {
	revocation :=
		newTestRevocation(t)

	originalID :=
		revocation.ID

	authorizer, err :=
		wallet.New()

	if err != nil {
		t.Fatal(err)
	}

	if err :=
		revocation.AddApproval(
			authorizer.Address,
			authorizer.PublicKeyHex(),
			authorizer.PrivateKey,
		); err != nil {

		t.Fatal(err)
	}

	if revocation.ID != originalID {
		t.Fatal(
			"revocation ID changed after approval",
		)
	}

	if CalculateRevocationID(
		revocation,
	) != originalID {

		t.Fatal(
			"revocation calculation depends on approvals",
		)
	}
}

func TestRevocationSupportsIndependentApprovals(
	t *testing.T,
) {
	revocation :=
		newTestRevocation(t)

	first, err := wallet.New()
	if err != nil {
		t.Fatal(err)
	}

	second, err := wallet.New()
	if err != nil {
		t.Fatal(err)
	}

	for _, authorizer := range []*wallet.Wallet{
		first,
		second,
	} {

		if err :=
			revocation.AddApproval(
				authorizer.Address,
				authorizer.PublicKeyHex(),
				authorizer.PrivateKey,
			); err != nil {

			t.Fatal(err)
		}
	}

	if len(revocation.Approvals) != 2 {
		t.Fatalf(
			"expected 2 approvals, got %d",
			len(revocation.Approvals),
		)
	}

	for _, approval := range revocation.Approvals {

		if err :=
			ValidateRevocationApproval(
				revocation,
				approval,
			); err != nil {

			t.Fatal(err)
		}
	}
}

func TestRevocationRejectsDuplicateApprover(
	t *testing.T,
) {
	revocation :=
		newTestRevocation(t)

	authorizer, err :=
		wallet.New()

	if err != nil {
		t.Fatal(err)
	}

	if err :=
		revocation.AddApproval(
			authorizer.Address,
			authorizer.PublicKeyHex(),
			authorizer.PrivateKey,
		); err != nil {

		t.Fatal(err)
	}

	if err :=
		revocation.AddApproval(
			authorizer.Address,
			authorizer.PublicKeyHex(),
			authorizer.PrivateKey,
		); err == nil {

		t.Fatal(
			"expected duplicate revocation approver to fail",
		)
	}
}

func TestRevocationApprovalCannotBeReused(
	t *testing.T,
) {
	first :=
		newTestRevocation(t)

	authorizer, err :=
		wallet.New()

	if err != nil {
		t.Fatal(err)
	}

	if err :=
		first.AddApproval(
			authorizer.Address,
			authorizer.PublicKeyHex(),
			authorizer.PrivateKey,
		); err != nil {

		t.Fatal(err)
	}

	approval :=
		first.Approvals[0]

	secondGrant :=
		newTestGrant(t)

	secondGrant.Nonce++
	secondGrant.ID =
		CalculateGrantID(
			secondGrant,
		)

	second :=
		NewRevocation(
			testChainID,
			consensus.ReservedPoolTreasury,
			secondGrant.ID,
		)

	if err :=
		ValidateRevocationApproval(
			second,
			approval,
		); err == nil {

		t.Fatal(
			"expected revocation approval replay to fail",
		)
	}
}

func TestRevocationRejectsInvalidGrantID(
	t *testing.T,
) {
	revocation :=
		NewRevocation(
			testChainID,
			consensus.ReservedPoolTreasury,
			"not-a-grant-id",
		)

	if err :=
		ValidateRevocation(
			revocation,
		); err == nil {

		t.Fatal(
			"expected invalid grant ID to fail",
		)
	}
}
