package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"prism/internal/blockchain"
	"prism/internal/reserved"
	"prism/internal/wallet"
)

func writeTestNodeChainConfig(
	t *testing.T,
	path string,
	config blockchain.ChainConfig,
) {
	t.Helper()

	data, err :=
		json.MarshalIndent(
			config,
			"",
			"  ",
		)

	if err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(
		path,
		data,
		0600,
	); err != nil {
		t.Fatal(err)
	}
}

func TestLoadNodeChainConfigCanonicalizes(
	t *testing.T,
) {
	first, err := wallet.New()
	if err != nil {
		t.Fatal(err)
	}

	second, err := wallet.New()
	if err != nil {
		t.Fatal(err)
	}

	config := blockchain.ChainConfig{
		ReservedAuthorities: reserved.AuthorityPolicy{
			Treasury: []string{
				second.Address,
				first.Address,
			},
		},
	}

	path :=
		filepath.Join(
			t.TempDir(),
			"chain-config.json",
		)

	writeTestNodeChainConfig(
		t,
		path,
		config,
	)

	loaded, err :=
		loadNodeChainConfig(path)

	if err != nil {
		t.Fatal(err)
	}

	if loaded == nil {
		t.Fatal(
			"expected loaded chain config",
		)
	}

	expected := []string{
		first.Address,
		second.Address,
	}

	sort.Strings(expected)

	got :=
		loaded.ReservedAuthorities.Treasury

	if len(got) != len(expected) {
		t.Fatalf(
			"expected %d treasury authorities, got %d",
			len(expected),
			len(got),
		)
	}

	for index := range expected {
		if got[index] != expected[index] {
			t.Fatal(
				"chain config was not canonicalized",
			)
		}
	}
}

func TestLoadNodeChainConfigRejectsNull(
	t *testing.T,
) {
	path :=
		filepath.Join(
			t.TempDir(),
			"chain-config.json",
		)

	if err := os.WriteFile(
		path,
		[]byte("null"),
		0600,
	); err != nil {
		t.Fatal(err)
	}

	if _, err :=
		loadNodeChainConfig(path); err == nil {

		t.Fatal(
			"expected null chain config to fail",
		)
	}
}

func TestLoadNodeChainConfigRejectsUnknownFields(
	t *testing.T,
) {
	path :=
		filepath.Join(
			t.TempDir(),
			"chain-config.json",
		)

	if err := os.WriteFile(
		path,
		[]byte(
			`{"reserved_authorities":{},"unknown":true}`,
		),
		0600,
	); err != nil {
		t.Fatal(err)
	}

	if _, err :=
		loadNodeChainConfig(path); err == nil {

		t.Fatal(
			"expected unknown chain config field to fail",
		)
	}
}

func TestLoadOrCreateP2PStateAppliesAndPinsChainConfig(
	t *testing.T,
) {
	firstAuthority, err :=
		wallet.New()

	if err != nil {
		t.Fatal(err)
	}

	secondAuthority, err :=
		wallet.New()

	if err != nil {
		t.Fatal(err)
	}

	configOne := blockchain.ChainConfig{
		ReservedAuthorities: reserved.AuthorityPolicy{
			Treasury: []string{
				firstAuthority.Address,
			},
		},
	}

	configTwo := blockchain.ChainConfig{
		ReservedAuthorities: reserved.AuthorityPolicy{
			Treasury: []string{
				secondAuthority.Address,
			},
		},
	}

	root := t.TempDir()

	dataPath :=
		filepath.Join(
			root,
			"node",
		)

	configOnePath :=
		filepath.Join(
			root,
			"config-one.json",
		)

	configTwoPath :=
		filepath.Join(
			root,
			"config-two.json",
		)

	writeTestNodeChainConfig(
		t,
		configOnePath,
		configOne,
	)

	writeTestNodeChainConfig(
		t,
		configTwoPath,
		configTwo,
	)

	chain, _, _, created, err :=
		loadOrCreateP2PState(
			dataPath,
			configOnePath,
		)

	if err != nil {
		t.Fatal(err)
	}

	if !created {
		t.Fatal(
			"expected new configured node state",
		)
	}

	expectedCommitment, err :=
		configOne.Commitment()

	if err != nil {
		t.Fatal(err)
	}

	actualCommitment, err :=
		chain.Config.Commitment()

	if err != nil {
		t.Fatal(err)
	}

	if actualCommitment !=
		expectedCommitment {

		t.Fatal(
			"created node did not persist requested chain config",
		)
	}

	loaded, _, _, created, err :=
		loadOrCreateP2PState(
			dataPath,
			configOnePath,
		)

	if err != nil {
		t.Fatal(err)
	}

	if created {
		t.Fatal(
			"expected existing configured node state",
		)
	}

	loadedCommitment, err :=
		loaded.Config.Commitment()

	if err != nil {
		t.Fatal(err)
	}

	if loadedCommitment !=
		expectedCommitment {

		t.Fatal(
			"reloaded node changed chain config",
		)
	}

	if _, _, _, _, err :=
		loadOrCreateP2PState(
			dataPath,
			configTwoPath,
		); err == nil {

		t.Fatal(
			"expected mismatched startup chain config to fail",
		)
	}
}

func TestLoadOrCreateP2PStateDefaultsToDenyAll(
	t *testing.T,
) {
	dataPath :=
		filepath.Join(
			t.TempDir(),
			"node",
		)

	chain, _, _, created, err :=
		loadOrCreateP2PState(
			dataPath,
			"",
		)

	if err != nil {
		t.Fatal(err)
	}

	if !created {
		t.Fatal(
			"expected new node state",
		)
	}

	if !chain.Config.IsLegacy() {
		t.Fatal(
			"expected default reserved authorities to remain deny-all",
		)
	}
}
