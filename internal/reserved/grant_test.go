package reserved

import (
	"testing"

	"prism/internal/consensus"
	"prism/internal/wallet"
)

func newTestGrant(
	t *testing.T,
) Grant {
	t.Helper()

	recipient, err := wallet.New()
	if err != nil {
		t.Fatal(err)
	}

	return NewGrant(
		testChainID,
		1,
		consensus.ReservedPoolTreasury,
		recipient.Address,
		100,
	)
}

func TestGrantIDDoesNotDependOnApprovals(
	t *testing.T,
) {
	grant := newTestGrant(t)

	originalID := grant.ID

	authorizer, err := wallet.New()
	if err != nil {
		t.Fatal(err)
	}

	if err :=
		grant.AddApproval(
			authorizer.Address,
			authorizer.PublicKeyHex(),
			authorizer.PrivateKey,
		); err != nil {

		t.Fatal(err)
	}

	if grant.ID != originalID {
		t.Fatal(
			"grant ID changed after approval",
		)
	}

	if CalculateGrantID(grant) != originalID {
		t.Fatal(
			"grant calculation depends on approvals",
		)
	}
}

func TestGrantSupportsIndependentApprovals(
	t *testing.T,
) {
	grant := newTestGrant(t)

	first, err := wallet.New()
	if err != nil {
		t.Fatal(err)
	}

	second, err := wallet.New()
	if err != nil {
		t.Fatal(err)
	}

	if err :=
		grant.AddApproval(
			first.Address,
			first.PublicKeyHex(),
			first.PrivateKey,
		); err != nil {

		t.Fatal(err)
	}

	if err :=
		grant.AddApproval(
			second.Address,
			second.PublicKeyHex(),
			second.PrivateKey,
		); err != nil {

		t.Fatal(err)
	}

	if len(grant.Approvals) != 2 {
		t.Fatalf(
			"expected 2 approvals, got %d",
			len(grant.Approvals),
		)
	}

	for _, approval := range grant.Approvals {
		if err :=
			ValidateApproval(
				grant,
				approval,
			); err != nil {

			t.Fatal(err)
		}
	}
}

func TestGrantRejectsDuplicateApprover(
	t *testing.T,
) {
	grant := newTestGrant(t)

	authorizer, err := wallet.New()
	if err != nil {
		t.Fatal(err)
	}

	if err :=
		grant.AddApproval(
			authorizer.Address,
			authorizer.PublicKeyHex(),
			authorizer.PrivateKey,
		); err != nil {

		t.Fatal(err)
	}

	if err :=
		grant.AddApproval(
			authorizer.Address,
			authorizer.PublicKeyHex(),
			authorizer.PrivateKey,
		); err == nil {

		t.Fatal(
			"expected duplicate approver to fail",
		)
	}
}

func TestGrantApprovalRejectsGrantTampering(
	t *testing.T,
) {
	grant := newTestGrant(t)

	authorizer, err := wallet.New()
	if err != nil {
		t.Fatal(err)
	}

	if err :=
		grant.AddApproval(
			authorizer.Address,
			authorizer.PublicKeyHex(),
			authorizer.PrivateKey,
		); err != nil {

		t.Fatal(err)
	}

	approval := grant.Approvals[0]

	grant.Amount++

	if err :=
		ValidateApproval(
			grant,
			approval,
		); err == nil {

		t.Fatal(
			"expected tampered grant to fail",
		)
	}
}

func TestGrantApprovalCannotBeReusedForAnotherGrant(
	t *testing.T,
) {
	firstGrant := newTestGrant(t)

	authorizer, err := wallet.New()
	if err != nil {
		t.Fatal(err)
	}

	if err :=
		firstGrant.AddApproval(
			authorizer.Address,
			authorizer.PublicKeyHex(),
			authorizer.PrivateKey,
		); err != nil {

		t.Fatal(err)
	}

	approval := firstGrant.Approvals[0]

	secondGrant := firstGrant
	secondGrant.Nonce++
	secondGrant.ID =
		CalculateGrantID(secondGrant)
	secondGrant.Approvals = nil

	if err :=
		ValidateApproval(
			secondGrant,
			approval,
		); err == nil {

		t.Fatal(
			"expected approval replay on another grant to fail",
		)
	}
}
