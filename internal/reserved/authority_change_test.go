package reserved

import (
	"strings"
	"testing"

	"prism/internal/consensus"
	"prism/internal/wallet"
)

func TestAuthorityChangeIDDeterministic(
	t *testing.T,
) {
	target, err := wallet.New()
	if err != nil {
		t.Fatal(err)
	}

	first :=
		NewAuthorityChange(
			"prism-authority-test",
			1,
			consensus.ReservedPoolTreasury,
			AuthorityChangeAdd,
			target.Address,
		)

	second :=
		NewAuthorityChange(
			"prism-authority-test",
			1,
			consensus.ReservedPoolTreasury,
			AuthorityChangeAdd,
			target.Address,
		)

	if first.ID != second.ID {
		t.Fatal(
			"expected deterministic authority change ID",
		)
	}
}

func TestAuthorityChangeAddApproval(
	t *testing.T,
) {
	target, err := wallet.New()
	if err != nil {
		t.Fatal(err)
	}

	authorizer, err := wallet.New()
	if err != nil {
		t.Fatal(err)
	}

	change :=
		NewAuthorityChange(
			"prism-authority-test",
			1,
			consensus.ReservedPoolTreasury,
			AuthorityChangeAdd,
			target.Address,
		)

	if err :=
		change.AddApproval(
			authorizer.Address,
			authorizer.PublicKeyHex(),
			authorizer.PrivateKey,
		); err != nil {

		t.Fatal(err)
	}

	if len(change.Approvals) != 1 {
		t.Fatalf(
			"expected one approval, got %d",
			len(change.Approvals),
		)
	}

	if err :=
		ValidateAuthorityChangeApproval(
			change,
			change.Approvals[0],
		); err != nil {

		t.Fatal(err)
	}
}

func TestAuthorityChangeRejectsDuplicateApproval(
	t *testing.T,
) {
	target, err := wallet.New()
	if err != nil {
		t.Fatal(err)
	}

	authorizer, err := wallet.New()
	if err != nil {
		t.Fatal(err)
	}

	change :=
		NewAuthorityChange(
			"prism-authority-test",
			1,
			consensus.ReservedPoolEcosystem,
			AuthorityChangeRemove,
			target.Address,
		)

	if err :=
		change.AddApproval(
			authorizer.Address,
			authorizer.PublicKeyHex(),
			authorizer.PrivateKey,
		); err != nil {

		t.Fatal(err)
	}

	err =
		change.AddApproval(
			authorizer.Address,
			authorizer.PublicKeyHex(),
			authorizer.PrivateKey,
		)

	if err == nil {
		t.Fatal(
			"expected duplicate approval to fail",
		)
	}

	if !strings.Contains(
		err.Error(),
		"already approved",
	) {
		t.Fatalf(
			"unexpected duplicate approval error: %v",
			err,
		)
	}
}

func TestAuthorityChangeRejectsTamperedPayload(
	t *testing.T,
) {
	target, err := wallet.New()
	if err != nil {
		t.Fatal(err)
	}

	otherTarget, err := wallet.New()
	if err != nil {
		t.Fatal(err)
	}

	authorizer, err := wallet.New()
	if err != nil {
		t.Fatal(err)
	}

	change :=
		NewAuthorityChange(
			"prism-authority-test",
			1,
			consensus.ReservedPoolTeam,
			AuthorityChangeAdd,
			target.Address,
		)

	if err :=
		change.AddApproval(
			authorizer.Address,
			authorizer.PublicKeyHex(),
			authorizer.PrivateKey,
		); err != nil {

		t.Fatal(err)
	}

	approval :=
		change.Approvals[0]

	change.Authority =
		otherTarget.Address

	change.ID =
		CalculateAuthorityChangeID(change)

	err =
		ValidateAuthorityChangeApproval(
			change,
			approval,
		)

	if err == nil {
		t.Fatal(
			"expected tampered authority change approval to fail",
		)
	}

	if !strings.Contains(
		err.Error(),
		"invalid reserved authority change approval signature",
	) {
		t.Fatalf(
			"unexpected tamper error: %v",
			err,
		)
	}
}

func TestAuthorityChangeRejectsInvalidAction(
	t *testing.T,
) {
	target, err := wallet.New()
	if err != nil {
		t.Fatal(err)
	}

	change :=
		NewAuthorityChange(
			"prism-authority-test",
			1,
			consensus.ReservedPoolLiquidity,
			AuthorityChangeAction("replace-everyone"),
			target.Address,
		)

	err =
		ValidateAuthorityChange(change)

	if err == nil {
		t.Fatal(
			"expected invalid authority change action to fail",
		)
	}
}

func TestAuthorityChangeRejectsZeroNonce(
	t *testing.T,
) {
	target, err := wallet.New()
	if err != nil {
		t.Fatal(err)
	}

	change :=
		NewAuthorityChange(
			"prism-authority-test",
			0,
			consensus.ReservedPoolTreasury,
			AuthorityChangeAdd,
			target.Address,
		)

	err =
		ValidateAuthorityChange(change)

	if err == nil {
		t.Fatal(
			"expected zero authority change nonce to fail",
		)
	}
}
