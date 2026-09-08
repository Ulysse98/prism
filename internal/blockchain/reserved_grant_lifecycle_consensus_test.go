package blockchain

import (
	"testing"

	"prism/internal/consensus"
	"prism/internal/reserved"
	"prism/internal/wallet"
)

func signedLifecycleGrantForBlockchain(
	t *testing.T,
	bc *Blockchain,
	authorities []*wallet.Wallet,
	nonce uint64,
	notBefore uint64,
	expiresAt uint64,
) reserved.Grant {
	t.Helper()

	recipient, err :=
		wallet.New()

	if err != nil {
		t.Fatal(err)
	}

	chainID, err :=
		bc.ChainID()

	if err != nil {
		t.Fatal(err)
	}

	grant :=
		reserved.NewGrantWithWindow(
			chainID,
			nonce,
			consensus.ReservedPoolTreasury,
			recipient.Address,
			100,
			notBefore,
			expiresAt,
		)

	for _, authority := range authorities[:2] {

		if err :=
			grant.AddApproval(
				authority.Address,
				authority.PublicKeyHex(),
				authority.PrivateKey,
			); err != nil {

			t.Fatal(err)
		}
	}

	return grant
}

func TestReservedGrantLifecycleRejectsBeforeActivationInConsensus(
	t *testing.T,
) {
	bc, pos, validator, authorities :=
		thresholdGrantBlockchain(t)

	grant :=
		signedLifecycleGrantForBlockchain(
			t,
			bc,
			authorities,
			1,
			2,
			10,
		)

	appendThresholdGrantTestBlock(
		bc,
		validator,
		grant,
	)

	if _, err :=
		bc.GetState(); err == nil {

		t.Fatal(
			"expected premature grant state reconstruction to fail",
		)
	}

	if bc.ValidateChain(pos) {
		t.Fatal(
			"expected premature lifecycle grant to fail consensus",
		)
	}
}

func TestReservedGrantLifecycleAcceptsActivationBoundaryInConsensus(
	t *testing.T,
) {
	bc, pos, validator, authorities :=
		thresholdGrantBlockchain(t)

	grant :=
		signedLifecycleGrantForBlockchain(
			t,
			bc,
			authorities,
			1,
			1,
			1,
		)

	appendThresholdGrantTestBlock(
		bc,
		validator,
		grant,
	)

	state, err :=
		bc.GetState()

	if err != nil {
		t.Fatal(err)
	}

	if state.Balances[grant.Recipient] !=
		grant.Amount {

		t.Fatalf(
			"expected lifecycle grant balance %d, got %d",
			grant.Amount,
			state.Balances[grant.Recipient],
		)
	}

	if !bc.ValidateChain(pos) {
		t.Fatal(
			"expected activation-boundary lifecycle grant to validate",
		)
	}
}
