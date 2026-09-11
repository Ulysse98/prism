package reserved

import (
	"encoding/json"
	"strings"
	"testing"

	"prism/internal/consensus"
	"prism/internal/wallet"
)

func TestGovernanceSnapshotRoundTripPreservesPendingProposal(
	t *testing.T,
) {
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

	policy := AuthorityPolicy{
		Treasury: []string{
			authorityA.Address,
			authorityB.Address,
		},
		TreasuryThreshold: 2,
	}

	const chainID = "prism-pending-snapshot-test"

	state, err :=
		NewGovernanceStateForChain(
			chainID,
			policy,
		)

	if err != nil {
		t.Fatal(err)
	}

	proposal :=
		NewAuthorityProposal(
			chainID,
			1,
			consensus.ReservedPoolTreasury,
			AuthorityChangeAdd,
			target.Address,
		)

	for _, authority := range []*wallet.Wallet{
		authorityA,
		authorityB,
	} {
		if err := proposal.AddApproval(
			authority.Address,
			authority.PublicKeyHex(),
			authority.PrivateKey,
		); err != nil {
			t.Fatal(err)
		}
	}

	if err := state.QueueAuthorityProposal(
		proposal,
		chainID,
		10,
		5,
	); err != nil {
		t.Fatal(err)
	}

	snapshot, err :=
		state.Snapshot()

	if err != nil {
		t.Fatal(err)
	}

	data, err :=
		json.Marshal(
			snapshot,
		)

	if err != nil {
		t.Fatal(err)
	}

	var decoded GovernanceSnapshot

	if err := json.Unmarshal(
		data,
		&decoded,
	); err != nil {
		t.Fatal(err)
	}

	restored, err :=
		GovernanceStateFromSnapshot(
			decoded,
		)

	if err != nil {
		t.Fatal(err)
	}

	pending, exists :=
		restored.GetPendingAuthorityProposal(
			proposal.ID,
		)

	if !exists {
		t.Fatal(
			"restored governance state lost pending proposal",
		)
	}

	if pending.ProposalHeight != 10 {
		t.Fatalf(
			"unexpected restored proposal height: got=%d expected=10",
			pending.ProposalHeight,
		)
	}

	if pending.DelayBlocks != 5 {
		t.Fatalf(
			"unexpected restored delay: got=%d expected=5",
			pending.DelayBlocks,
		)
	}

	if pending.ExecuteAfterHeight != 15 {
		t.Fatalf(
			"unexpected restored execution height: got=%d expected=15",
			pending.ExecuteAfterHeight,
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

	if authorized {
		t.Fatal(
			"restored pending proposal activated authority",
		)
	}

	// Queueing must still not have consumed replay protection.
	if err :=
		restored.Replay.ValidateAuthorityChangeNext(
			proposal.Change,
		); err != nil {

		t.Fatalf(
			"restored pending proposal consumed replay state: %v",
			err,
		)
	}
}

func TestGovernanceSnapshotRejectsTamperedPendingProposal(
	t *testing.T,
) {
	authority, err := wallet.New()
	if err != nil {
		t.Fatal(err)
	}

	target, err := wallet.New()
	if err != nil {
		t.Fatal(err)
	}

	policy := AuthorityPolicy{
		Treasury: []string{
			authority.Address,
		},
		TreasuryThreshold: 1,
	}

	const chainID = "prism-pending-snapshot-tamper-test"

	state, err :=
		NewGovernanceStateForChain(
			chainID,
			policy,
		)

	if err != nil {
		t.Fatal(err)
	}

	proposal :=
		NewAuthorityProposal(
			chainID,
			1,
			consensus.ReservedPoolTreasury,
			AuthorityChangeAdd,
			target.Address,
		)

	if err := proposal.AddApproval(
		authority.Address,
		authority.PublicKeyHex(),
		authority.PrivateKey,
	); err != nil {
		t.Fatal(err)
	}

	if err := state.QueueAuthorityProposal(
		proposal,
		chainID,
		10,
		5,
	); err != nil {
		t.Fatal(err)
	}

	snapshot, err :=
		state.Snapshot()

	if err != nil {
		t.Fatal(err)
	}

	if len(snapshot.PendingProposals) != 1 {
		t.Fatalf(
			"expected one pending proposal in snapshot, got %d",
			len(snapshot.PendingProposals),
		)
	}

	snapshot.PendingProposals[0].ExecuteAfterHeight = 14

	_, err =
		GovernanceStateFromSnapshot(
			snapshot,
		)

	if err == nil {
		t.Fatal(
			"expected tampered pending proposal snapshot to fail",
		)
	}

	if !strings.Contains(
		err.Error(),
		"invalid stored pending authority proposal",
	) {
		t.Fatalf(
			"unexpected pending snapshot error: %v",
			err,
		)
	}
}
