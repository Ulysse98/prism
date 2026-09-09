package blockchain

import (
	"testing"

	"prism/internal/reserved"
	"prism/internal/wallet"
)

func signedThresholdRevocationForBlockchain(
	t *testing.T,
	bc *Blockchain,
	grant reserved.Grant,
	authorities []*wallet.Wallet,
	approvalCount int,
) reserved.Revocation {
	t.Helper()

	chainID, err :=
		bc.ChainID()

	if err != nil {
		t.Fatal(err)
	}

	revocation :=
		reserved.NewRevocation(
			chainID,
			grant.Pool,
			grant.ID,
		)

	for i := 0; i < approvalCount; i++ {
		if err :=
			revocation.AddApproval(
				authorities[i].Address,
				authorities[i].PublicKeyHex(),
				authorities[i].PrivateKey,
			); err != nil {

			t.Fatal(err)
		}
	}

	return revocation
}

func TestAddReservedRevocationBlock(
	t *testing.T,
) {
	bc, pos, validator, authorities :=
		thresholdGrantBlockchain(t)

	grant :=
		signedThresholdGrantForBlockchain(
			t,
			bc,
			authorities,
			2,
		)

	revocation :=
		signedThresholdRevocationForBlockchain(
			t,
			bc,
			grant,
			authorities,
			2,
		)

	block, err :=
		bc.AddReservedRevocationBlock(
			[]reserved.Revocation{
				revocation,
			},
			validator.Address,
			pos,
		)

	if err != nil {
		t.Fatal(err)
	}

	if len(block.ReservedRevocations) != 1 {
		t.Fatalf(
			"expected one reserved revocation, got %d",
			len(block.ReservedRevocations),
		)
	}

	if block.ReservedRevocations[0].ID !=
		revocation.ID {

		t.Fatal(
			"produced block did not preserve revocation",
		)
	}

	if !bc.ValidateChain(pos) {
		t.Fatal(
			"expected reserved revocation block to validate",
		)
	}
}

func TestRevokedGrantCannotExecute(
	t *testing.T,
) {
	bc, pos, validator, authorities :=
		thresholdGrantBlockchain(t)

	grant :=
		signedThresholdGrantForBlockchain(
			t,
			bc,
			authorities,
			2,
		)

	revocation :=
		signedThresholdRevocationForBlockchain(
			t,
			bc,
			grant,
			authorities,
			2,
		)

	if _, err :=
		bc.AddReservedRevocationBlock(
			[]reserved.Revocation{
				revocation,
			},
			validator.Address,
			pos,
		); err != nil {

		t.Fatal(err)
	}

	before :=
		len(bc.Blocks)

	if _, err :=
		bc.AddReservedGrantBlock(
			[]reserved.Grant{
				grant,
			},
			validator.Address,
			pos,
		); err == nil {

		t.Fatal(
			"expected revoked grant execution to fail",
		)
	}

	if len(bc.Blocks) != before {
		t.Fatal(
			"failed revoked grant execution mutated chain",
		)
	}
}

func TestExecutedGrantCannotBeRevoked(
	t *testing.T,
) {
	bc, pos, validator, authorities :=
		thresholdGrantBlockchain(t)

	grant :=
		signedThresholdGrantForBlockchain(
			t,
			bc,
			authorities,
			2,
		)

	if _, err :=
		bc.AddReservedGrantBlock(
			[]reserved.Grant{
				grant,
			},
			validator.Address,
			pos,
		); err != nil {

		t.Fatal(err)
	}

	revocation :=
		signedThresholdRevocationForBlockchain(
			t,
			bc,
			grant,
			authorities,
			2,
		)

	before :=
		len(bc.Blocks)

	if _, err :=
		bc.AddReservedRevocationBlock(
			[]reserved.Revocation{
				revocation,
			},
			validator.Address,
			pos,
		); err == nil {

		t.Fatal(
			"expected executed grant revocation to fail",
		)
	}

	if len(bc.Blocks) != before {
		t.Fatal(
			"failed revocation mutated chain",
		)
	}
}

func TestAddReservedRevocationBlockCopiesApprovals(
	t *testing.T,
) {
	bc, pos, validator, authorities :=
		thresholdGrantBlockchain(t)

	grant :=
		signedThresholdGrantForBlockchain(
			t,
			bc,
			authorities,
			2,
		)

	revocation :=
		signedThresholdRevocationForBlockchain(
			t,
			bc,
			grant,
			authorities,
			2,
		)

	originalSignature :=
		revocation.Approvals[0].Signature

	block, err :=
		bc.AddReservedRevocationBlock(
			[]reserved.Revocation{
				revocation,
			},
			validator.Address,
			pos,
		)

	if err != nil {
		t.Fatal(err)
	}

	revocation.Approvals[0].Signature =
		"tampered-input"

	stored :=
		&bc.Blocks[len(bc.Blocks)-1]

	if stored.ReservedRevocations[0].
		Approvals[0].
		Signature != originalSignature {

		t.Fatal(
			"input mutation changed stored revocation",
		)
	}

	block.ReservedRevocations[0].
		Approvals[0].
		Signature = "tampered-return"

	if stored.ReservedRevocations[0].
		Approvals[0].
		Signature != originalSignature {

		t.Fatal(
			"returned block mutation changed stored revocation",
		)
	}

	if !bc.ValidateChain(pos) {
		t.Fatal(
			"stored chain became invalid after external mutation",
		)
	}
}
