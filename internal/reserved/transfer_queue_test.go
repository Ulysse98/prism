package reserved

import (
	"strings"
	"testing"

	"prism/internal/consensus"
	"prism/internal/wallet"
)

func signedTreasuryTransfer(
	t *testing.T,
	authority *wallet.Wallet,
	nonce uint64,
) ReservedTransferProposal {
	t.Helper()

	proposal :=
		NewReservedTransferProposal(
			"prism-devnet",
			nonce,
			consensus.ReservedPoolTreasury,
			"recipient",
			100,
		)

	if err :=
		proposal.AddApproval(
			authority.Address,
			authority.PublicKeyHex(),
			authority.PrivateKey,
		); err != nil {

		t.Fatal(err)
	}

	return proposal
}

func TestGovernanceStateQueuesReservedTransfer(
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

	pending, exists :=
		state.GetPendingReservedTransferProposal(
			proposal.ID,
		)

	if !exists {
		t.Fatal(
			"expected transfer proposal to be pending",
		)
	}

	if pending.ExecuteAfterHeight != 105 {
		t.Fatalf(
			"unexpected execution height: %d",
			pending.ExecuteAfterHeight,
		)
	}
}

func TestGovernanceStateReservedTransferQueueClonesApprovals(
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

	proposal.Approvals[0].Signature = "tampered"

	pending, exists :=
		state.GetPendingReservedTransferProposal(
			proposal.ID,
		)

	if !exists {
		t.Fatal(
			"expected transfer proposal to be pending",
		)
	}

	if pending.Proposal.Approvals[0].Signature ==
		"tampered" {

		t.Fatal(
			"pending transfer approvals were not cloned",
		)
	}
}

func TestGovernanceStateRejectsDuplicateReservedTransferProposal(
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

	err =
		state.QueueReservedTransferProposal(
			proposal,
			"prism-devnet",
			101,
			5,
		)

	if err == nil {
		t.Fatal(
			"expected duplicate pending transfer to be rejected",
		)
	}

	if !strings.Contains(
		err.Error(),
		"already pending",
	) {
		t.Fatalf(
			"unexpected error: %v",
			err,
		)
	}
}

func TestGovernanceStateRejectsNonIncreasingReservedTransferNonce(
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

	second :=
		signedTreasuryTransfer(
			t,
			authority,
			2,
		)

	if err :=
		state.QueueReservedTransferProposal(
			second,
			"prism-devnet",
			100,
			5,
		); err != nil {

		t.Fatal(err)
	}

	first :=
		signedTreasuryTransfer(
			t,
			authority,
			1,
		)

	err =
		state.QueueReservedTransferProposal(
			first,
			"prism-devnet",
			101,
			5,
		)

	if err == nil {
		t.Fatal(
			"expected non-increasing nonce to be rejected",
		)
	}

	if !strings.Contains(
		err.Error(),
		"nonce is not increasing",
	) {
		t.Fatalf(
			"unexpected error: %v",
			err,
		)
	}
}

func TestGovernanceStateRejectsReservedTransferWithoutQuorum(
	t *testing.T,
) {
	authorityA :=
		mustReservedTransferWallet(t)

	authorityB :=
		mustReservedTransferWallet(t)

	state, err :=
		NewGovernanceStateForChain(
			"prism-devnet",
			AuthorityPolicy{
				Treasury: []string{
					authorityA.Address,
					authorityB.Address,
				},
				TreasuryThreshold: 2,
			},
		)

	if err != nil {
		t.Fatal(err)
	}

	proposal :=
		signedTreasuryTransfer(
			t,
			authorityA,
			1,
		)

	err =
		state.QueueReservedTransferProposal(
			proposal,
			"prism-devnet",
			100,
			5,
		)

	if err == nil {
		t.Fatal(
			"expected missing quorum to be rejected",
		)
	}

	if !strings.Contains(
		err.Error(),
		"approval threshold not met",
	) {
		t.Fatalf(
			"unexpected error: %v",
			err,
		)
	}
}
