package reserved

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
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

func TestTimelockedAuthorityChangeCommitsActivationHeight(
	t *testing.T,
) {
	target, err := wallet.New()
	if err != nil {
		t.Fatal(err)
	}

	first :=
		NewTimelockedAuthorityChange(
			"prism-authority-test",
			1,
			consensus.ReservedPoolTreasury,
			AuthorityChangeAdd,
			target.Address,
			100,
		)

	second :=
		NewTimelockedAuthorityChange(
			"prism-authority-test",
			1,
			consensus.ReservedPoolTreasury,
			AuthorityChangeAdd,
			target.Address,
			101,
		)

	if first.ID == second.ID {
		t.Fatal(
			"activation height must affect authority change ID",
		)
	}
}

func TestLegacyAuthorityChangeRetainsV1ID(
	t *testing.T,
) {
	target, err := wallet.New()
	if err != nil {
		t.Fatal(err)
	}

	const chainID = "prism-authority-test"
	const nonce uint64 = 1

	change :=
		NewAuthorityChange(
			chainID,
			nonce,
			consensus.ReservedPoolTreasury,
			AuthorityChangeAdd,
			target.Address,
		)

	if change.ActivationHeight != 0 {
		t.Fatalf(
			"legacy authority change activation height = %d, want 0",
			change.ActivationHeight,
		)
	}

	expectedPayload :=
		fmt.Sprintf(
			"reserved-authority-change-v1|%s|%d|%s|%s|%s",
			chainID,
			nonce,
			consensus.ReservedPoolTreasury,
			AuthorityChangeAdd,
			target.Address,
		)

	hash :=
		sha256.Sum256(
			[]byte(expectedPayload),
		)

	expectedID :=
		hex.EncodeToString(hash[:])

	if change.ID != expectedID {
		t.Fatalf(
			"legacy authority change ID changed: got %s want %s",
			change.ID,
			expectedID,
		)
	}
}

func TestTimelockedAuthorityChangeApprovalBindsActivationHeight(
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
		NewTimelockedAuthorityChange(
			"prism-authority-test",
			1,
			consensus.ReservedPoolTreasury,
			AuthorityChangeAdd,
			target.Address,
			100,
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

	change.ActivationHeight = 101

	change.ID =
		CalculateAuthorityChangeID(change)

	err =
		ValidateAuthorityChangeApproval(
			change,
			approval,
		)

	if err == nil {
		t.Fatal(
			"expected changed activation height to invalidate approval",
		)
	}

	if !strings.Contains(
		err.Error(),
		"invalid reserved authority change approval signature",
	) {
		t.Fatalf(
			"unexpected activation-height tamper error: %v",
			err,
		)
	}
}

func TestTimelockedAuthorityChangeIDDeterministic(
	t *testing.T,
) {
	target, err := wallet.New()
	if err != nil {
		t.Fatal(err)
	}

	first :=
		NewTimelockedAuthorityChange(
			"prism-authority-test",
			7,
			consensus.ReservedPoolEcosystem,
			AuthorityChangeRemove,
			target.Address,
			250,
		)

	second :=
		NewTimelockedAuthorityChange(
			"prism-authority-test",
			7,
			consensus.ReservedPoolEcosystem,
			AuthorityChangeRemove,
			target.Address,
			250,
		)

	if first.ID != second.ID {
		t.Fatal(
			"expected deterministic timelocked authority change ID",
		)
	}
}

func TestLegacyAndTimelockedAuthorityChangesHaveDifferentIDs(
	t *testing.T,
) {
	target, err := wallet.New()
	if err != nil {
		t.Fatal(err)
	}

	legacy :=
		NewAuthorityChange(
			"prism-authority-test",
			1,
			consensus.ReservedPoolTreasury,
			AuthorityChangeAdd,
			target.Address,
		)

	timelocked :=
		NewTimelockedAuthorityChange(
			"prism-authority-test",
			1,
			consensus.ReservedPoolTreasury,
			AuthorityChangeAdd,
			target.Address,
			100,
		)

	if legacy.ID == timelocked.ID {
		t.Fatal(
			"legacy and timelocked authority changes must use different IDs",
		)
	}
}
