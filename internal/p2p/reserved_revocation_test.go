package p2p

import (
	"testing"

	"prism/internal/reserved"
)

func TestReservedRevocationBlockConvergesAcrossPeers(
	t *testing.T,
) {
	source,
		pos,
		validator,
		recipient,
		authorities,
		makeReceiver :=
		lifecycleP2PSetup(t)

	nodeB :=
		makeReceiver(
			"revocation-node-b",
		)

	nodeC :=
		makeReceiver(
			"revocation-node-c",
		)

	grant :=
		signedLifecycleP2PGrant(
			t,
			source,
			recipient,
			authorities,
			1,
			1,
			3,
		)

	chainID, err :=
		source.ChainID()

	if err != nil {
		t.Fatal(err)
	}

	revocation :=
		reserved.NewRevocation(
			chainID,
			grant.Pool,
			grant.ID,
		)

	for _, authority := range authorities[:2] {

		if err :=
			revocation.AddApproval(
				authority.Address,
				authority.PublicKeyHex(),
				authority.PrivateKey,
			); err != nil {

			t.Fatal(err)
		}
	}

	block, err :=
		source.AddReservedRevocationBlock(
			[]reserved.Revocation{
				revocation,
			},
			validator.Address,
			pos,
		)

	if err != nil {
		t.Fatal(err)
	}

	wireBlock :=
		lifecycleWireBlock(
			t,
			chainID,
			block,
		)

	if len(wireBlock.ReservedRevocations) != 1 {
		t.Fatalf(
			"expected one wire revocation, got %d",
			len(wireBlock.ReservedRevocations),
		)
	}

	wireRevocation :=
		wireBlock.ReservedRevocations[0]

	if wireRevocation.ID != revocation.ID {
		t.Fatal(
			"wire encoding changed revocation ID",
		)
	}

	if wireRevocation.GrantID != grant.ID {
		t.Fatal(
			"wire encoding changed revoked grant ID",
		)
	}

	if len(wireRevocation.Approvals) != 2 {
		t.Fatalf(
			"expected two wire revocation approvals, got %d",
			len(wireRevocation.Approvals),
		)
	}

	for _, server := range []*Server{
		nodeB,
		nodeC,
	} {

		appended, err :=
			server.acceptBlock(
				wireBlock,
			)

		if err != nil {
			t.Fatal(err)
		}

		if !appended {
			t.Fatal(
				"expected reserved revocation block to append",
			)
		}

		tip :=
			server.Chain.Blocks[len(server.Chain.Blocks)-1]

		if tip.Hash != block.Hash {
			t.Fatal(
				"peer changed revocation block hash",
			)
		}

		if len(tip.ReservedRevocations) != 1 {
			t.Fatal(
				"peer lost reserved revocation",
			)
		}

		received :=
			tip.ReservedRevocations[0]

		if received.ID != revocation.ID {
			t.Fatal(
				"peer changed revocation ID",
			)
		}

		if received.GrantID != grant.ID {
			t.Fatal(
				"peer changed revoked grant ID",
			)
		}

		if len(received.Approvals) !=
			len(revocation.Approvals) {

			t.Fatalf(
				"expected %d revocation approvals, got %d",
				len(revocation.Approvals),
				len(received.Approvals),
			)
		}

		for i := range revocation.Approvals {

			if received.Approvals[i].Authorizer !=
				revocation.Approvals[i].Authorizer {

				t.Fatalf(
					"peer changed revocation approver at index %d",
					i,
				)
			}

			if received.Approvals[i].Signature !=
				revocation.Approvals[i].Signature {

				t.Fatalf(
					"peer changed revocation signature at index %d",
					i,
				)
			}
		}

		emission, err :=
			server.Chain.ReservedEmission()

		if err != nil {
			t.Fatal(err)
		}

		if emission != 0 {
			t.Fatalf(
				"revocation consumed reserved emission: %d",
				emission,
			)
		}

		if !server.Chain.ValidateChain(pos) {
			t.Fatal(
				"peer revocation chain failed validation",
			)
		}

		before :=
			len(server.Chain.Blocks)

		if _, err :=
			server.Chain.AddReservedGrantBlock(
				[]reserved.Grant{
					grant,
				},
				validator.Address,
				pos,
			); err == nil {

			t.Fatal(
				"peer accepted execution of revoked grant",
			)
		}

		if len(server.Chain.Blocks) != before {
			t.Fatal(
				"rejected revoked grant mutated peer chain",
			)
		}
	}
}
