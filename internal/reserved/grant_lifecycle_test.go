package reserved

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"testing"

	"prism/internal/consensus"
	"prism/internal/wallet"
)

func lifecycleTestRecipient(
	t *testing.T,
) string {
	t.Helper()

	recipient, err := wallet.New()
	if err != nil {
		t.Fatal(err)
	}

	return recipient.Address
}

func TestLegacyGrantKeepsV1ID(
	t *testing.T,
) {
	recipient :=
		lifecycleTestRecipient(t)

	grant :=
		NewGrant(
			testChainID,
			1,
			consensus.ReservedPoolTreasury,
			recipient,
			100,
		)

	payload :=
		fmt.Sprintf(
			"reserved-grant-v1|%s|%d|%s|%s|%d",
			grant.ChainID,
			grant.Nonce,
			grant.Pool,
			grant.Recipient,
			grant.Amount,
		)

	hash :=
		sha256.Sum256(
			[]byte(payload),
		)

	expected :=
		hex.EncodeToString(
			hash[:],
		)

	if grant.ID != expected {
		t.Fatalf(
			"expected legacy v1 grant ID %s, got %s",
			expected,
			grant.ID,
		)
	}
}

func TestLifecycleWindowChangesGrantID(
	t *testing.T,
) {
	recipient :=
		lifecycleTestRecipient(t)

	first :=
		NewGrantWithWindow(
			testChainID,
			1,
			consensus.ReservedPoolTreasury,
			recipient,
			100,
			5,
			10,
		)

	second :=
		NewGrantWithWindow(
			testChainID,
			1,
			consensus.ReservedPoolTreasury,
			recipient,
			100,
			5,
			11,
		)

	if first.ID == second.ID {
		t.Fatal(
			"grant lifecycle window must affect grant ID",
		)
	}
}

func TestValidateGrantAtHeightAcceptsLifecycleBoundaries(
	t *testing.T,
) {
	grant :=
		NewGrantWithWindow(
			testChainID,
			1,
			consensus.ReservedPoolTreasury,
			lifecycleTestRecipient(t),
			100,
			5,
			10,
		)

	if err :=
		ValidateGrantAtHeight(
			grant,
			5,
		); err != nil {

		t.Fatal(err)
	}

	if err :=
		ValidateGrantAtHeight(
			grant,
			10,
		); err != nil {

		t.Fatal(err)
	}
}

func TestValidateGrantAtHeightRejectsBeforeActivation(
	t *testing.T,
) {
	grant :=
		NewGrantWithWindow(
			testChainID,
			1,
			consensus.ReservedPoolTreasury,
			lifecycleTestRecipient(t),
			100,
			5,
			10,
		)

	if err :=
		ValidateGrantAtHeight(
			grant,
			4,
		); err == nil {

		t.Fatal(
			"expected grant before activation height to fail",
		)
	}
}

func TestValidateGrantAtHeightRejectsExpiredGrant(
	t *testing.T,
) {
	grant :=
		NewGrantWithWindow(
			testChainID,
			1,
			consensus.ReservedPoolTreasury,
			lifecycleTestRecipient(t),
			100,
			5,
			10,
		)

	if err :=
		ValidateGrantAtHeight(
			grant,
			11,
		); err == nil {

		t.Fatal(
			"expected expired grant to fail",
		)
	}
}

func TestValidateGrantRejectsReversedLifecycleWindow(
	t *testing.T,
) {
	grant :=
		NewGrantWithWindow(
			testChainID,
			1,
			consensus.ReservedPoolTreasury,
			lifecycleTestRecipient(t),
			100,
			10,
			5,
		)

	if err :=
		ValidateGrant(grant); err == nil {

		t.Fatal(
			"expected reversed lifecycle window to fail",
		)
	}
}
