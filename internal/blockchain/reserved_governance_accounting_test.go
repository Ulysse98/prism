package blockchain

import (
	"testing"

	"prism/internal/consensus"
	"prism/internal/reserved"
	"prism/internal/wallet"
)

func reservedGovernanceAccountingFixture(
	t *testing.T,
) (
	*Blockchain,
	*wallet.Wallet,
	*wallet.Wallet,
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
			"alice": 1000,
		},
	)
	if err != nil {
		t.Fatal(err)
	}

	bc.Config = ChainConfig{
		ReservedAuthorities: reserved.AuthorityPolicy{
			Treasury: []string{
				authorityA.Address,
			},
		},
	}

	return bc,
		authorityA,
		authorityB
}

func signedAuthorityAdditionForAccounting(
	t *testing.T,
	bc *Blockchain,
	currentAuthority *wallet.Wallet,
	newAuthority *wallet.Wallet,
	nonce uint64,
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
		newAuthority.Address,
	)

	if err := change.AddApproval(
		currentAuthority.Address,
		currentAuthority.PublicKeyHex(),
		currentAuthority.PrivateKey,
	); err != nil {
		t.Fatal(err)
	}

	return change
}

func TestReservedAccountingActivatesAuthorityChangeNextBlock(
	t *testing.T,
) {
	bc, authorityA, authorityB :=
		reservedGovernanceAccountingFixture(t)

	change :=
		signedAuthorityAdditionForAccounting(
			t,
			bc,
			authorityA,
			authorityB,
			1,
		)

	bc.Blocks = append(
		bc.Blocks,
		Block{
			Height: 1,
			AuthorityChanges: []reserved.AuthorityChange{
				change,
			},
		},
	)

	authorization :=
		signedReservedReplayAuthorization(
			t,
			bc,
			authorityB,
			1,
			250,
		)

	bc.Blocks = append(
		bc.Blocks,
		Block{
			Height: 2,
			ReservedAuthorizations: []reserved.Authorization{
				authorization,
			},
		},
	)

	state, err := bc.GetReservedAccountingState()
	if err != nil {
		t.Fatal(err)
	}

	if state.Usage.Treasury != 250 {
		t.Fatalf(
			"expected treasury usage 250 after newly activated authority, got %d",
			state.Usage.Treasury,
		)
	}
}

func TestReservedAccountingDoesNotActivateAuthorityChangeWithinSameBlock(
	t *testing.T,
) {
	bc, authorityA, authorityB :=
		reservedGovernanceAccountingFixture(t)

	change :=
		signedAuthorityAdditionForAccounting(
			t,
			bc,
			authorityA,
			authorityB,
			1,
		)

	authorization :=
		signedReservedReplayAuthorization(
			t,
			bc,
			authorityB,
			1,
			250,
		)

	bc.Blocks = append(
		bc.Blocks,
		Block{
			Height: 1,
			ReservedAuthorizations: []reserved.Authorization{
				authorization,
			},
			AuthorityChanges: []reserved.AuthorityChange{
				change,
			},
		},
	)

	if _, err := bc.GetReservedAccountingState(); err == nil {
		t.Fatal(
			"expected newly added authority to be unauthorized within the same block",
		)
	}
}

func TestReservedAccountingOldAuthorityRemainsValidBeforeChange(
	t *testing.T,
) {
	bc, authorityA, authorityB :=
		reservedGovernanceAccountingFixture(t)

	change :=
		signedAuthorityAdditionForAccounting(
			t,
			bc,
			authorityA,
			authorityB,
			1,
		)

	authorization :=
		signedReservedReplayAuthorization(
			t,
			bc,
			authorityA,
			1,
			100,
		)

	bc.Blocks = append(
		bc.Blocks,
		Block{
			Height: 1,
			ReservedAuthorizations: []reserved.Authorization{
				authorization,
			},
			AuthorityChanges: []reserved.AuthorityChange{
				change,
			},
		},
	)

	state, err := bc.GetReservedAccountingState()
	if err != nil {
		t.Fatal(err)
	}

	if state.Usage.Treasury != 100 {
		t.Fatalf(
			"expected treasury usage 100, got %d",
			state.Usage.Treasury,
		)
	}
}

func TestReservedAccountingRemovedAuthorityCannotAuthorizeNextBlock(
	t *testing.T,
) {
	bc, authorityA, authorityB :=
		reservedGovernanceAccountingFixture(t)

	// Block 1:
	// authority A adds authority B.
	addChange :=
		signedAuthorityAdditionForAccounting(
			t,
			bc,
			authorityA,
			authorityB,
			1,
		)

	bc.Blocks = append(
		bc.Blocks,
		Block{
			Height: 1,
			AuthorityChanges: []reserved.AuthorityChange{
				addChange,
			},
		},
	)

	chainID, err := bc.ChainID()
	if err != nil {
		t.Fatal(err)
	}

	// Block 2:
	// authority B is now active and removes authority A.
	removeChange := reserved.NewAuthorityChange(
		chainID,
		2,
		consensus.ReservedPoolTreasury,
		reserved.AuthorityChangeRemove,
		authorityA.Address,
	)

	if err := removeChange.AddApproval(
		authorityB.Address,
		authorityB.PublicKeyHex(),
		authorityB.PrivateKey,
	); err != nil {
		t.Fatal(err)
	}

	bc.Blocks = append(
		bc.Blocks,
		Block{
			Height: 2,
			AuthorityChanges: []reserved.AuthorityChange{
				removeChange,
			},
		},
	)

	// Block 3:
	// authority A has been removed and must no longer
	// be able to authorize a treasury emission.
	authorization :=
		signedReservedReplayAuthorization(
			t,
			bc,
			authorityA,
			1,
			100,
		)

	bc.Blocks = append(
		bc.Blocks,
		Block{
			Height: 3,
			ReservedAuthorizations: []reserved.Authorization{
				authorization,
			},
		},
	)

	if _, err := bc.GetReservedAccountingState(); err == nil {
		t.Fatal(
			"expected removed authority to be unauthorized in the following block",
		)
	}
}
