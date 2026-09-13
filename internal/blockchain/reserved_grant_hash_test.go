package blockchain

import (
	"encoding/json"
	"strings"
	"testing"

	"prism/internal/consensus"
	"prism/internal/reserved"
)

func reservedGrantHashTestGrant() reserved.Grant {
	return reserved.Grant{
		ID:        "grant-id",
		ChainID:   "prism-test-chain",
		Nonce:     1,
		Pool:      consensus.ReservedPoolTreasury,
		Recipient: "recipient",
		Amount:    100,
		Approvals: []reserved.Approval{
			{
				Authorizer: "authority-a",
				PublicKey:  "public-key-a",
				Signature:  "signature-a",
			},
			{
				Authorizer: "authority-b",
				PublicKey:  "public-key-b",
				Signature:  "signature-b",
			},
		},
	}
}

func TestEmptyReservedGrantsPreserveHash(
	t *testing.T,
) {
	nilBlock :=
		reservedHashTestBlock()

	emptyBlock :=
		nilBlock

	emptyBlock.ReservedGrants =
		[]reserved.Grant{}

	nilHash :=
		CalculateHash(nilBlock)

	emptyHash :=
		CalculateHash(emptyBlock)

	if nilHash != emptyHash {
		t.Fatal(
			"empty reserved grants changed legacy block hash",
		)
	}
}

func TestReservedGrantChangesBlockHash(
	t *testing.T,
) {
	block :=
		reservedHashTestBlock()

	legacyHash :=
		CalculateHash(block)

	block.ReservedGrants =
		[]reserved.Grant{
			reservedGrantHashTestGrant(),
		}

	grantHash :=
		CalculateHash(block)

	if legacyHash == grantHash {
		t.Fatal(
			"reserved grant must change block hash",
		)
	}
}

func TestReservedGrantContentsAreHashed(
	t *testing.T,
) {
	block :=
		reservedHashTestBlock()

	block.ReservedGrants =
		[]reserved.Grant{
			reservedGrantHashTestGrant(),
		}

	originalHash :=
		CalculateHash(block)

	block.ReservedGrants[0].Amount++

	tamperedHash :=
		CalculateHash(block)

	if originalHash == tamperedHash {
		t.Fatal(
			"reserved grant contents must be committed by block hash",
		)
	}
}

func TestReservedGrantApprovalsAreHashed(
	t *testing.T,
) {
	block :=
		reservedHashTestBlock()

	block.ReservedGrants =
		[]reserved.Grant{
			reservedGrantHashTestGrant(),
		}

	originalHash :=
		CalculateHash(block)

	block.ReservedGrants[0].
		Approvals[0].
		Signature = "tampered-signature"

	tamperedHash :=
		CalculateHash(block)

	if originalHash == tamperedHash {
		t.Fatal(
			"reserved grant approvals must be committed by block hash",
		)
	}
}

func TestReservedAuthorizationHashPathSurvivesGrantUpgrade(
	t *testing.T,
) {
	block :=
		reservedHashTestBlock()

	block.ReservedAuthorizations =
		[]reserved.Authorization{
			reservedHashTestAuthorization(),
		}

	before :=
		CalculateHash(block)

	block.ReservedGrants =
		[]reserved.Grant{}

	after :=
		CalculateHash(block)

	if before != after {
		t.Fatal(
			"empty grant slice changed v0.21 reserved authorization hash",
		)
	}
}

func TestReservedGrantLifecycleHeightsAreHashed(
	t *testing.T,
) {
	grant :=
		reservedGrantHashTestGrant()

	grant.NotBeforeHeight = 5
	grant.ExpiresAtHeight = 10

	block :=
		reservedHashTestBlock()

	block.ReservedGrants =
		[]reserved.Grant{
			grant,
		}

	originalHash :=
		CalculateHash(block)

	block.ReservedGrants[0].
		NotBeforeHeight = 6

	if CalculateHash(block) ==
		originalHash {

		t.Fatal(
			"reserved grant activation height must affect block hash",
		)
	}

	block.ReservedGrants[0] =
		grant

	block.ReservedGrants[0].
		ExpiresAtHeight = 11

	if CalculateHash(block) ==
		originalHash {

		t.Fatal(
			"reserved grant expiration height must affect block hash",
		)
	}
}

func TestLegacyReservedGrantJSONOmitsLifecycleFields(
	t *testing.T,
) {
	grant :=
		reservedGrantHashTestGrant()

	data, err :=
		json.Marshal(grant)

	if err != nil {
		t.Fatal(err)
	}

	encoded :=
		string(data)

	if strings.Contains(
		encoded,
		"not_before_height",
	) {
		t.Fatal(
			"zero activation height changed v0.22 grant JSON",
		)
	}

	if strings.Contains(
		encoded,
		"expires_at_height",
	) {
		t.Fatal(
			"zero expiration height changed v0.22 grant JSON",
		)
	}
}
