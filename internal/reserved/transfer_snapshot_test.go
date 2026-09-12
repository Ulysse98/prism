package reserved

import (
	"strings"
	"testing"

	"prism/internal/consensus"
)

func TestGovernanceSnapshotRoundTripsReservedTransfer(
	t *testing.T,
) {
	authority :=
		mustReservedTransferWallet(t)

	state, err :=
		NewGovernanceStateForChain(
			"prism-devnet",
			AuthorityPolicy{
				Treasury: []string{
					authority.Address,
				},
				TreasuryThreshold: 1,
			},
		)

	if err != nil {
		t.Fatal(err)
	}

	proposal :=
		signedTreasuryTransfer(
			t,
			authority,
			1,
		)

	if err :=
		state.QueueReservedTransferProposal(
			proposal,
			"prism-devnet",
			100,
			5,
		); err != nil {

		t.Fatal(err)
	}

	snapshot, err :=
		state.Snapshot()

	if err != nil {
		t.Fatal(err)
	}

	if len(snapshot.PendingTransfers) != 1 {
		t.Fatalf(
			"expected one pending transfer in snapshot, got %d",
			len(snapshot.PendingTransfers),
		)
	}

	restored, err :=
		GovernanceStateFromSnapshot(
			snapshot,
		)

	if err != nil {
		t.Fatal(err)
	}

	pending, exists :=
		restored.GetPendingReservedTransferProposal(
			proposal.ID,
		)

	if !exists {
		t.Fatal(
			"restored pending transfer not found",
		)
	}

	if pending.ExecuteAfterHeight != 105 {
		t.Fatalf(
			"unexpected restored execute-after height: %d",
			pending.ExecuteAfterHeight,
		)
	}

	if len(pending.Proposal.Approvals) != 1 {
		t.Fatalf(
			"unexpected restored approval count: %d",
			len(pending.Proposal.Approvals),
		)
	}
}

func TestGovernanceSnapshotSortsReservedTransfersDeterministically(
	t *testing.T,
) {
	authority :=
		mustReservedTransferWallet(t)

	state, err :=
		NewGovernanceStateForChain(
			"prism-devnet",
			AuthorityPolicy{
				Treasury: []string{
					authority.Address,
				},
				Ecosystem: []string{
					authority.Address,
				},
				TreasuryThreshold:  1,
				EcosystemThreshold: 1,
			},
		)

	if err != nil {
		t.Fatal(err)
	}

	treasury :=
		signedTreasuryTransfer(
			t,
			authority,
			1,
		)

	ecosystem :=
		NewReservedTransferProposal(
			"prism-devnet",
			1,
			consensus.ReservedPoolEcosystem,
			"recipient",
			100,
		)

	if err :=
		ecosystem.AddApproval(
			authority.Address,
			authority.PublicKeyHex(),
			authority.PrivateKey,
		); err != nil {

		t.Fatal(err)
	}

	if err :=
		state.QueueReservedTransferProposal(
			treasury,
			"prism-devnet",
			100,
			5,
		); err != nil {

		t.Fatal(err)
	}

	if err :=
		state.QueueReservedTransferProposal(
			ecosystem,
			"prism-devnet",
			101,
			5,
		); err != nil {

		t.Fatal(err)
	}

	snapshot, err :=
		state.Snapshot()

	if err != nil {
		t.Fatal(err)
	}

	if len(snapshot.PendingTransfers) != 2 {
		t.Fatalf(
			"expected two pending transfers, got %d",
			len(snapshot.PendingTransfers),
		)
	}

	if snapshot.PendingTransfers[0].Proposal.ID >
		snapshot.PendingTransfers[1].Proposal.ID {

		t.Fatal(
			"pending transfers are not sorted by proposal ID",
		)
	}
}

func TestGovernanceStateFromSnapshotRejectsDuplicateReservedTransfer(
	t *testing.T,
) {
	state, err :=
		NewGovernanceState(
			AuthorityPolicy{},
		)

	if err != nil {
		t.Fatal(err)
	}

	snapshot, err :=
		state.Snapshot()

	if err != nil {
		t.Fatal(err)
	}

	proposal :=
		NewReservedTransferProposal(
			"",
			1,
			consensus.ReservedPoolTreasury,
			"recipient",
			100,
		)

	proposal.ChainID = "prism-devnet"
	proposal.ID =
		CalculateReservedTransferProposalID(
			proposal,
		)

	pending, err :=
		NewPendingReservedTransferProposal(
			proposal,
			100,
			5,
		)

	if err != nil {
		t.Fatal(err)
	}

	snapshot.ChainID = "prism-devnet"
	snapshot.PendingTransfers =
		[]PendingReservedTransferProposal{
			pending,
			pending,
		}

	_, err =
		GovernanceStateFromSnapshot(
			snapshot,
		)

	if err == nil {
		t.Fatal(
			"expected duplicate stored transfer to be rejected",
		)
	}

	if !strings.Contains(
		err.Error(),
		"duplicate stored pending reserved transfer proposal",
	) {
		t.Fatalf(
			"unexpected error: %v",
			err,
		)
	}
}

func TestGovernanceStateFromSnapshotRejectsDuplicateReservedTransferNonce(
	t *testing.T,
) {
	state, err :=
		NewGovernanceState(
			AuthorityPolicy{},
		)

	if err != nil {
		t.Fatal(err)
	}

	snapshot, err :=
		state.Snapshot()

	if err != nil {
		t.Fatal(err)
	}

	first :=
		NewReservedTransferProposal(
			"prism-devnet",
			1,
			consensus.ReservedPoolTreasury,
			"recipient-a",
			100,
		)

	second :=
		NewReservedTransferProposal(
			"prism-devnet",
			1,
			consensus.ReservedPoolTreasury,
			"recipient-b",
			100,
		)

	firstPending, err :=
		NewPendingReservedTransferProposal(
			first,
			100,
			5,
		)

	if err != nil {
		t.Fatal(err)
	}

	secondPending, err :=
		NewPendingReservedTransferProposal(
			second,
			101,
			5,
		)

	if err != nil {
		t.Fatal(err)
	}

	snapshot.ChainID = "prism-devnet"
	snapshot.PendingTransfers =
		[]PendingReservedTransferProposal{
			firstPending,
			secondPending,
		}

	_, err =
		GovernanceStateFromSnapshot(
			snapshot,
		)

	if err == nil {
		t.Fatal(
			"expected duplicate transfer nonce to be rejected",
		)
	}

	if !strings.Contains(
		err.Error(),
		"duplicate stored pending reserved transfer proposal nonce",
	) {
		t.Fatalf(
			"unexpected error: %v",
			err,
		)
	}
}
