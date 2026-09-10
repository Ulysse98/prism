package blockchain

import (
	"testing"
	"time"

	"prism/internal/consensus"
	"prism/internal/reserved"
	"prism/internal/wallet"
)

func governanceConsensusFixture(
	t *testing.T,
) (
	*Blockchain,
	*consensus.ProofOfStake,
	[]*wallet.Wallet,
) {
	t.Helper()

	validator, err := wallet.New()
	if err != nil {
		t.Fatal(err)
	}

	authorityA, err := wallet.New()
	if err != nil {
		t.Fatal(err)
	}

	authorityB, err := wallet.New()
	if err != nil {
		t.Fatal(err)
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
			Treasury: []string{
				authorityA.Address,
				authorityB.Address,
			},
			TreasuryThreshold: 2,
		},
	}

	return bc,
		pos,
		[]*wallet.Wallet{
			authorityA,
			authorityB,
		}
}

func appendGovernanceConsensusBlock(
	t *testing.T,
	bc *Blockchain,
	pos *consensus.ProofOfStake,
	change reserved.AuthorityChange,
) {
	t.Helper()

	previous :=
		bc.Blocks[len(bc.Blocks)-1]

	nextHeight :=
		previous.Height + 1

	proposer, err := pos.SelectProposer(
		previous.Hash,
		nextHeight,
	)
	if err != nil {
		t.Fatal(err)
	}

	block := Block{
		Height: nextHeight,
		Timestamp: time.Unix(
			int64(nextHeight+100),
			0,
		).UTC(),
		PreviousHash: previous.Hash,
		Proposer:     proposer.Address,
		Reward:       consensus.DefaultProposerReward,
		AuthorityChanges: []reserved.AuthorityChange{
			change,
		},
	}

	block.Hash = CalculateHash(block)

	bc.Blocks = append(
		bc.Blocks,
		block,
	)
}

func TestAuthorityChangeParticipatesInConsensus(
	t *testing.T,
) {
	bc, pos, authorities :=
		governanceConsensusFixture(t)

	target, err := wallet.New()
	if err != nil {
		t.Fatal(err)
	}

	chainID, err := bc.ChainID()
	if err != nil {
		t.Fatal(err)
	}

	change := reserved.NewAuthorityChange(
		chainID,
		1,
		consensus.ReservedPoolTreasury,
		reserved.AuthorityChangeAdd,
		target.Address,
	)

	for _, authority := range authorities {
		if err := change.AddApproval(
			authority.Address,
			authority.PublicKeyHex(),
			authority.PrivateKey,
		); err != nil {
			t.Fatal(err)
		}
	}

	appendGovernanceConsensusBlock(
		t,
		bc,
		pos,
		change,
	)

	if !bc.ValidateChain(pos) {
		t.Fatal(
			"expected valid authority change chain to pass consensus",
		)
	}

	state, err := bc.GetGovernanceState()
	if err != nil {
		t.Fatal(err)
	}

	authorized, err :=
		state.CurrentPolicy.IsAuthorized(
			consensus.ReservedPoolTreasury,
			target.Address,
		)

	if err != nil {
		t.Fatal(err)
	}

	if !authorized {
		t.Fatal(
			"consensus-valid authority change was not reconstructed",
		)
	}
}

func TestWrongChainAuthorityChangeFailsConsensus(
	t *testing.T,
) {
	bc, pos, authorities :=
		governanceConsensusFixture(t)

	target, err := wallet.New()
	if err != nil {
		t.Fatal(err)
	}

	change := reserved.NewAuthorityChange(
		"prism-wrong-chain",
		1,
		consensus.ReservedPoolTreasury,
		reserved.AuthorityChangeAdd,
		target.Address,
	)

	for _, authority := range authorities {
		if err := change.AddApproval(
			authority.Address,
			authority.PublicKeyHex(),
			authority.PrivateKey,
		); err != nil {
			t.Fatal(err)
		}
	}

	appendGovernanceConsensusBlock(
		t,
		bc,
		pos,
		change,
	)

	if bc.ValidateChain(pos) {
		t.Fatal(
			"expected wrong-chain authority change to fail consensus",
		)
	}
}

func TestBelowThresholdAuthorityChangeFailsConsensus(
	t *testing.T,
) {
	bc, pos, authorities :=
		governanceConsensusFixture(t)

	target, err := wallet.New()
	if err != nil {
		t.Fatal(err)
	}

	chainID, err := bc.ChainID()
	if err != nil {
		t.Fatal(err)
	}

	change := reserved.NewAuthorityChange(
		chainID,
		1,
		consensus.ReservedPoolTreasury,
		reserved.AuthorityChangeAdd,
		target.Address,
	)

	if err := change.AddApproval(
		authorities[0].Address,
		authorities[0].PublicKeyHex(),
		authorities[0].PrivateKey,
	); err != nil {
		t.Fatal(err)
	}

	appendGovernanceConsensusBlock(
		t,
		bc,
		pos,
		change,
	)

	if bc.ValidateChain(pos) {
		t.Fatal(
			"expected below-threshold authority change to fail consensus",
		)
	}
}
