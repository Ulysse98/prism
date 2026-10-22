package blockchain

import (
	"bytes"
	"encoding/json"
	"reflect"
	"testing"

	"prism/internal/compute"
	"prism/internal/escrow"
	"prism/internal/usefulwork"
)

func TestEscrowV065PostRefundCannotBeReopenedAfterReplay(t *testing.T) {
	scenarios := []struct {
		name       string
		expiry     uint64
		claimFirst bool
	}{
		{name: "refund_unclaimed", expiry: 2},
		{name: "refund_claimed", expiry: 3, claimFirst: true},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.name, func(t *testing.T) {
			source, alice, bob, task := escrowLifecycleFixtureV065(t, scenario.expiry)
			pos := escrowTestPoSV065(t, alice)

			chainID, err := source.ChainID()
			if err != nil {
				t.Fatal(err)
			}
			jobID := escrow.CalculateFundedJobID(
				task.ID, alice.Address, 10, 55,
			)

			claim, err := compute.SignClaimAuthorization(
				jobID, chainID, source.Blocks[0].Hash, bob,
			)
			if err != nil {
				t.Fatal(err)
			}

			proof, err := usefulwork.ExecuteCompute(
				task, jobID, chainID, source.Blocks[0].Hash, bob,
			)
			if err != nil {
				t.Fatal(err)
			}

			if scenario.claimFirst {
				if _, err := source.AddEscrowClaimBlockV065(
					claim, alice.Address, pos,
				); err != nil {
					t.Fatal(err)
				}
			}

			if _, err := source.AddEscrowRefundBlockV065(
				jobID, alice.Address, pos,
			); err != nil {
				t.Fatal(err)
			}

			encoded, err := json.Marshal(source)
			if err != nil {
				t.Fatal(err)
			}

			var restarted Blockchain
			if err := json.Unmarshal(encoded, &restarted); err != nil {
				t.Fatal(err)
			}

			for _, target := range []struct {
				name  string
				chain *Blockchain
			}{
				{"original", source},
				{"restored_from_json", &restarted},
			} {
				t.Run(target.name, func(t *testing.T) {
					chain := target.chain

					if !chain.ValidateChain(pos) {
						t.Fatal("refunded chain failed validation")
					}

					record, err := chain.GetEscrowRecordV065(jobID)
					if err != nil || record.Status != escrow.Refunded {
						t.Fatalf("unexpected escrow: %+v, %v", record, err)
					}

					locked, err := chain.GetEscrowLockedBalancesV065()
					if err != nil || locked[alice.Address] != 0 {
						t.Fatalf("funds still locked: %v, %v", locked, err)
					}

					baseline, err := json.Marshal(chain)
					if err != nil {
						t.Fatal(err)
					}
					stateBefore, err := chain.GetState()
					if err != nil {
						t.Fatal(err)
					}
					availableBefore, err := chain.AvailableBalanceOf(alice.Address)
					if err != nil {
						t.Fatal(err)
					}

					reject := func(name string, action func() error) {
						t.Helper()

						if err := action(); err == nil {
							t.Fatalf("%s accepted after REFUND", name)
						}

						after, err := json.Marshal(chain)
						if err != nil {
							t.Fatal(err)
						}
						if !bytes.Equal(baseline, after) {
							t.Fatalf("%s mutated blockchain", name)
						}

						stateAfter, err := chain.GetState()
						if err != nil ||
							!reflect.DeepEqual(stateBefore, stateAfter) {
							t.Fatalf("%s changed ledger: %v", name, err)
						}

						availableAfter, err := chain.AvailableBalanceOf(alice.Address)
						if err != nil || availableBefore != availableAfter {
							t.Fatalf("%s changed spendable balance: %v", name, err)
						}
					}

					reject("CLAIM", func() error {
						_, err := chain.AddEscrowClaimBlockV065(
							claim, alice.Address, pos,
						)
						return err
					})

					reject("RELEASE", func() error {
						_, err := chain.AddEscrowReleaseBlockV065(
							proof, alice.Address, pos,
						)
						return err
					})

					reject("REFUND", func() error {
						_, err := chain.AddEscrowRefundBlockV065(
							jobID, alice.Address, pos,
						)
						return err
					})

					if !chain.ValidateChain(pos) {
						t.Fatal("chain invalid after rejected operations")
					}
				})
			}
		})
	}
}
