package crosschain

import (
	"strings"
	"testing"
)

func settlementTestHex32(
	pair string,
) string {
	return "0x" +
		strings.Repeat(
			pair,
			32,
		)
}

func TestSettlementStorePersistsConfirmedArbitrum(
	t *testing.T,
) {
	dataDir := t.TempDir()

	store, err :=
		NewSettlementStore(
			dataDir,
		)
	if err != nil {
		t.Fatal(err)
	}

	input := Settlement{
		RegistryID: settlementTestHex32("ab"),
		Chain:      SettlementChainArbitrum,
		Status:     SettlementStatusConfirmed,
		TxHash:     settlementTestHex32("cd"),
		RegistryAddress: "0x" +
			strings.Repeat(
				"12",
				20,
			),
		BlockNumber: 311964958,
		ExplorerURL: "https://sepolia.arbiscan.io/tx/" +
			settlementTestHex32("cd"),
	}

	saved, err :=
		store.Upsert(input)
	if err != nil {
		t.Fatal(err)
	}

	if saved.UpdatedAt == "" {
		t.Fatal(
			"expected settlement update timestamp",
		)
	}

	reloaded, err :=
		NewSettlementStore(
			dataDir,
		)
	if err != nil {
		t.Fatal(err)
	}

	settlements, err :=
		reloaded.ForRegistry(
			input.RegistryID,
		)
	if err != nil {
		t.Fatal(err)
	}

	if len(settlements) != 1 {
		t.Fatalf(
			"expected 1 settlement, got %d",
			len(settlements),
		)
	}

	got := settlements[0]

	if got.Chain != input.Chain ||
		got.Status != input.Status ||
		got.TxHash != input.TxHash ||
		got.RegistryAddress !=
			input.RegistryAddress ||
		got.BlockNumber !=
			input.BlockNumber {

		t.Fatalf(
			"persisted settlement mismatch: %+v",
			got,
		)
	}
}

func TestSettlementStoreRejectsIncompleteConfirmation(
	t *testing.T,
) {
	store, err :=
		NewSettlementStore(
			t.TempDir(),
		)
	if err != nil {
		t.Fatal(err)
	}

	_, err =
		store.Upsert(
			Settlement{
				RegistryID: settlementTestHex32(
					"ab",
				),
				Chain:       SettlementChainArbitrum,
				Status:      SettlementStatusConfirmed,
				BlockNumber: 1,
			},
		)

	if err == nil {
		t.Fatal(
			"expected confirmed settlement without tx hash to fail",
		)
	}
}
