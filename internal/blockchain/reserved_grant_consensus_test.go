package blockchain

import (
	"testing"
	"time"

	"prism/internal/consensus"
	"prism/internal/reserved"
	"prism/internal/wallet"
)

func thresholdGrantBlockchain(
	t *testing.T,
) (
	*Blockchain,
	*consensus.ProofOfStake,
	*wallet.Wallet,
	[]*wallet.Wallet,
) {
	t.Helper()

	validator, err := wallet.New()
	if err != nil {
		t.Fatal(err)
	}

	authorities := make(
		[]*wallet.Wallet,
		3,
	)

	addresses := make(
		[]string,
		3,
	)

	for i := range authorities {
		authorities[i], err =
			wallet.New()

		if err != nil {
			t.Fatal(err)
		}

		addresses[i] =
			authorities[i].Address
	}

	bc, err := NewBlockchain(
		map[string]uint64{
			validator.Address: 1000,
		},
	)
	if err != nil {
		t.Fatal(err)
	}

	const stake uint64 = 10

	if err := bc.LockStake(
		validator.Address,
		stake,
	); err != nil {
		t.Fatal(err)
	}

	pos := consensus.NewProofOfStake()

	if err := pos.Register(
		validator.Address,
		stake,
	); err != nil {
		t.Fatal(err)
	}

	bc.Config = ChainConfig{
		ReservedAuthorities: reserved.AuthorityPolicy{
			Treasury:          addresses,
			TreasuryThreshold: 2,
		},
	}

	return bc,
		pos,
		validator,
		authorities
}

func signedThresholdGrantForBlockchain(
	t *testing.T,
	bc *Blockchain,
	authorities []*wallet.Wallet,
	approvalCount int,
) reserved.Grant {
	t.Helper()

	recipient, err := wallet.New()
	if err != nil {
		t.Fatal(err)
	}

	chainID, err := bc.ChainID()
	if err != nil {
		t.Fatal(err)
	}

	grant := reserved.NewGrant(
		chainID,
		1,
		consensus.ReservedPoolTreasury,
		recipient.Address,
		100,
	)

	for i := 0; i < approvalCount; i++ {
		if err := grant.AddApproval(
			authorities[i].Address,
			authorities[i].PublicKeyHex(),
			authorities[i].PrivateKey,
		); err != nil {
			t.Fatal(err)
		}
	}

	return grant
}

func appendThresholdGrantTestBlock(
	bc *Blockchain,
	validator *wallet.Wallet,
	grant reserved.Grant,
) {
	rewardPolicy :=
		consensus.DefaultRewardPolicy()

	block := Block{
		Height:       1,
		Timestamp:    time.Unix(2, 0).UTC(),
		PreviousHash: bc.Blocks[0].Hash,
		Proposer:     validator.Address,
		Reward:       rewardPolicy.ProposerReward,
		ReservedGrants: []reserved.Grant{
			grant,
		},
	}

	block.Hash =
		CalculateHash(block)

	bc.Blocks =
		append(
			bc.Blocks,
			block,
		)
}

func TestThresholdReservedGrantParticipatesInConsensus(
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

	appendThresholdGrantTestBlock(
		bc,
		validator,
		grant,
	)

	state, err := bc.GetState()
	if err != nil {
		t.Fatal(err)
	}

	if state.Balances[grant.Recipient] !=
		grant.Amount {

		t.Fatalf(
			"expected grant recipient balance %d, got %d",
			grant.Amount,
			state.Balances[grant.Recipient],
		)
	}

	if !bc.ValidateChain(pos) {
		t.Fatal(
			"expected threshold grant chain to validate",
		)
	}

	emission, err :=
		bc.ReservedEmission()

	if err != nil {
		t.Fatal(err)
	}

	if emission != grant.Amount {
		t.Fatalf(
			"expected reserved emission %d, got %d",
			grant.Amount,
			emission,
		)
	}
}

func TestThresholdReservedGrantBelowThresholdFailsConsensus(
	t *testing.T,
) {
	bc, pos, validator, authorities :=
		thresholdGrantBlockchain(t)

	grant :=
		signedThresholdGrantForBlockchain(
			t,
			bc,
			authorities,
			1,
		)

	appendThresholdGrantTestBlock(
		bc,
		validator,
		grant,
	)

	if _, err := bc.GetState(); err == nil {
		t.Fatal(
			"expected below-threshold grant state reconstruction to fail",
		)
	}

	if bc.ValidateChain(pos) {
		t.Fatal(
			"expected below-threshold grant chain to fail consensus",
		)
	}
}

func TestGenesisRejectsThresholdReservedGrant(
	t *testing.T,
) {
	bc, _, _, authorities :=
		thresholdGrantBlockchain(t)

	grant :=
		signedThresholdGrantForBlockchain(
			t,
			bc,
			authorities,
			2,
		)

	bc.Blocks[0].ReservedGrants =
		[]reserved.Grant{
			grant,
		}

	if _, err := bc.GetState(); err == nil {
		t.Fatal(
			"expected genesis reserved grant to fail",
		)
	}
}
