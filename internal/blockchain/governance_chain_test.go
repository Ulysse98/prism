package blockchain

import (
	"strings"
	"testing"
	"time"

	"prism/internal/consensus"
	"prism/internal/reserved"
	"prism/internal/wallet"
)

func governanceChainFixture(
	t *testing.T,
) (
	*Blockchain,
	[]*wallet.Wallet,
) {
	t.Helper()

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
			authorityA.Address: 100,
			authorityB.Address: 100,
		},
	)
	if err != nil {
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

	return bc, []*wallet.Wallet{
		authorityA,
		authorityB,
	}
}

func signedGovernanceChange(
	t *testing.T,
	bc *Blockchain,
	authorities []*wallet.Wallet,
	nonce uint64,
	target string,
) reserved.AuthorityChange {
	t.Helper()

	chainID, err := bc.ChainID()
	if err != nil {
		t.Fatal(err)
	}

	change := reserved.NewAuthorityChange(
		chainID,
		nonce,
		consensus.ReservedPoolTreasury,
		reserved.AuthorityChangeAdd,
		target,
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

	return change
}

func appendGovernanceTestBlock(
	bc *Blockchain,
	change reserved.AuthorityChange,
	height uint64,
) {
	previous := bc.Blocks[len(bc.Blocks)-1]

	block := Block{
		Height:       height,
		Timestamp:    time.Unix(int64(height+1), 0).UTC(),
		PreviousHash: previous.Hash,
		Proposer:     "governance-test-proposer",
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

func TestGetGovernanceStateReconstructsAuthorityChanges(
	t *testing.T,
) {
	bc, authorities :=
		governanceChainFixture(t)

	target, err := wallet.New()
	if err != nil {
		t.Fatal(err)
	}

	change := signedGovernanceChange(
		t,
		bc,
		authorities,
		1,
		target.Address,
	)

	appendGovernanceTestBlock(
		bc,
		change,
		1,
	)

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
			"authority change was not reconstructed from blockchain",
		)
	}
}

func TestGetGovernanceStateRejectsWrongChainID(
	t *testing.T,
) {
	bc, authorities :=
		governanceChainFixture(t)

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

	appendGovernanceTestBlock(
		bc,
		change,
		1,
	)

	_, err = bc.GetGovernanceState()
	if err == nil {
		t.Fatal(
			"expected wrong-chain authority change to fail",
		)
	}

	if !strings.Contains(
		err.Error(),
		"chain ID mismatch",
	) {
		t.Fatalf(
			"unexpected wrong-chain error: %v",
			err,
		)
	}
}

func TestGetGovernanceStateRejectsNonIncreasingNonce(
	t *testing.T,
) {
	bc, authorities :=
		governanceChainFixture(t)

	targetA, err := wallet.New()
	if err != nil {
		t.Fatal(err)
	}

	targetB, err := wallet.New()
	if err != nil {
		t.Fatal(err)
	}

	first := signedGovernanceChange(
		t,
		bc,
		authorities,
		2,
		targetA.Address,
	)

	second := signedGovernanceChange(
		t,
		bc,
		authorities,
		1,
		targetB.Address,
	)

	appendGovernanceTestBlock(
		bc,
		first,
		1,
	)

	appendGovernanceTestBlock(
		bc,
		second,
		2,
	)

	_, err = bc.GetGovernanceState()
	if err == nil {
		t.Fatal(
			"expected non-increasing authority change nonce to fail",
		)
	}

	if !strings.Contains(
		err.Error(),
		"nonce is not increasing",
	) {
		t.Fatalf(
			"unexpected nonce error: %v",
			err,
		)
	}
}

func TestGetGovernanceStateRejectsBelowThresholdChange(
	t *testing.T,
) {
	bc, authorities :=
		governanceChainFixture(t)

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

	// Only one approval for a 2-of-2 authority policy.
	if err := change.AddApproval(
		authorities[0].Address,
		authorities[0].PublicKeyHex(),
		authorities[0].PrivateKey,
	); err != nil {
		t.Fatal(err)
	}

	appendGovernanceTestBlock(
		bc,
		change,
		1,
	)

	_, err = bc.GetGovernanceState()
	if err == nil {
		t.Fatal(
			"expected below-threshold authority change to fail",
		)
	}

	if !strings.Contains(
		err.Error(),
		"approval threshold not met",
	) {
		t.Fatalf(
			"unexpected threshold error: %v",
			err,
		)
	}
}

func TestGetGovernanceStateRejectsGenesisAuthorityChange(
	t *testing.T,
) {
	bc, authorities :=
		governanceChainFixture(t)

	target, err := wallet.New()
	if err != nil {
		t.Fatal(err)
	}

	change := signedGovernanceChange(
		t,
		bc,
		authorities,
		1,
		target.Address,
	)

	bc.Blocks[0].AuthorityChanges =
		[]reserved.AuthorityChange{
			change,
		}

	_, err = bc.GetGovernanceState()
	if err == nil {
		t.Fatal(
			"expected genesis authority change to fail",
		)
	}

	if !strings.Contains(
		err.Error(),
		"genesis block cannot contain reserved authority changes",
	) {
		t.Fatalf(
			"unexpected genesis governance error: %v",
			err,
		)
	}
}
