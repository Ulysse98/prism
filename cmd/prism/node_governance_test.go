package main

import (
	"strings"
	"testing"

	"prism/internal/blockchain"
	"prism/internal/consensus"
	"prism/internal/reserved"
	"prism/internal/storage"
	"prism/internal/wallet"
)

func TestLoadOrCreateNodeGovernanceStateRestoresPersistedState(
	t *testing.T,
) {
	dataPath := t.TempDir()

	authorityA, err := wallet.New()
	if err != nil {
		t.Fatal(err)
	}

	authorityB, err := wallet.New()
	if err != nil {
		t.Fatal(err)
	}

	target, err := wallet.New()
	if err != nil {
		t.Fatal(err)
	}

	policy := reserved.AuthorityPolicy{
		Treasury: []string{
			authorityA.Address,
			authorityB.Address,
		},
		TreasuryThreshold: 2,
	}

	chain, err := blockchain.NewBlockchain(
		map[string]uint64{
			authorityA.Address: 100,
			authorityB.Address: 100,
		},
	)

	if err != nil {
		t.Fatal(err)
	}

	chain.Config = blockchain.ChainConfig{
		ReservedAuthorities: policy,
	}

	state, created, err :=
		loadOrCreateNodeGovernanceState(
			dataPath,
			chain,
		)

	if err != nil {
		t.Fatal(err)
	}

	if !created {
		t.Fatal(
			"expected governance state to be initialized",
		)
	}

	if !storage.GovernanceStateExists(
		dataPath,
	) {
		t.Fatal(
			"expected governance state file to exist",
		)
	}

	chainID, err := chain.ChainID()
	if err != nil {
		t.Fatal(err)
	}

	change :=
		reserved.NewAuthorityChange(
			chainID,
			1,
			consensus.ReservedPoolTreasury,
			reserved.AuthorityChangeAdd,
			target.Address,
		)

	for _, signer := range []*wallet.Wallet{
		authorityA,
		authorityB,
	} {
		if err := change.AddApproval(
			signer.Address,
			signer.PublicKeyHex(),
			signer.PrivateKey,
		); err != nil {
			t.Fatal(err)
		}
	}

	if err := state.ApplyAuthorityChange(
		change,
		chainID,
	); err != nil {
		t.Fatal(err)
	}

	if err := storage.SaveGovernanceState(
		dataPath,
		state,
	); err != nil {
		t.Fatal(err)
	}

	// Simulate another node process startup.
	state = nil

	restored, created, err :=
		loadOrCreateNodeGovernanceState(
			dataPath,
			chain,
		)

	if err != nil {
		t.Fatal(err)
	}

	if created {
		t.Fatal(
			"persisted governance state was incorrectly reinitialized",
		)
	}

	if restored.ChainID != chainID {
		t.Fatalf(
			"restored governance chain ID mismatch: expected=%s got=%s",
			chainID,
			restored.ChainID,
		)
	}

	authorized, err :=
		restored.CurrentPolicy.IsAuthorized(
			consensus.ReservedPoolTreasury,
			target.Address,
		)

	if err != nil {
		t.Fatal(err)
	}

	if !authorized {
		t.Fatal(
			"node restart lost runtime authority change",
		)
	}

	err =
		restored.Replay.ValidateAuthorityChangeNext(
			change,
		)

	if err == nil {
		t.Fatal(
			"node restart accepted replayed authority change",
		)
	}

	if !strings.Contains(
		err.Error(),
		"already used",
	) {
		t.Fatalf(
			"unexpected replay error after node restart: %v",
			err,
		)
	}
}

func TestLoadOrCreateNodeGovernanceStateRejectsWrongChain(
	t *testing.T,
) {
	dataPath := t.TempDir()

	authorityA, err := wallet.New()
	if err != nil {
		t.Fatal(err)
	}

	authorityB, err := wallet.New()
	if err != nil {
		t.Fatal(err)
	}

	policy := reserved.AuthorityPolicy{
		Treasury: []string{
			authorityA.Address,
			authorityB.Address,
		},
		TreasuryThreshold: 2,
	}

	chainA, err := blockchain.NewBlockchain(
		map[string]uint64{
			authorityA.Address: 100,
			authorityB.Address: 100,
		},
	)

	if err != nil {
		t.Fatal(err)
	}

	chainA.Config = blockchain.ChainConfig{
		ReservedAuthorities: policy,
	}

	_, created, err :=
		loadOrCreateNodeGovernanceState(
			dataPath,
			chainA,
		)

	if err != nil {
		t.Fatal(err)
	}

	if !created {
		t.Fatal(
			"expected governance state to be created",
		)
	}

	// Different genesis balances produce another network identity.
	chainB, err := blockchain.NewBlockchain(
		map[string]uint64{
			authorityA.Address: 200,
			authorityB.Address: 100,
		},
	)

	if err != nil {
		t.Fatal(err)
	}

	chainB.Config = blockchain.ChainConfig{
		ReservedAuthorities: policy,
	}

	_, _, err =
		loadOrCreateNodeGovernanceState(
			dataPath,
			chainB,
		)

	if err == nil {
		t.Fatal(
			"expected governance state from another chain to be rejected",
		)
	}

	if !strings.Contains(
		err.Error(),
		"governance chain ID mismatch",
	) {
		t.Fatalf(
			"unexpected wrong-chain governance error: %v",
			err,
		)
	}
}
