package reserved

import (
	"encoding/json"
	"strings"
	"testing"

	"prism/internal/consensus"
)

func TestGovernanceSnapshotAfterExecutionPreservesReplayAndClearsPending(
	t *testing.T,
) {
	state,
		proposal,
		target,
		authorities,
		chainID := governanceProposalFixture(
		t,
		1,
	)

	approveGovernanceProposal(
		t,
		&proposal,
		authorities...,
	)

	if err := state.QueueAuthorityProposal(
		proposal,
		chainID,
		10,
		5,
	); err != nil {
		t.Fatal(err)
	}

	if err := state.ExecuteAuthorityProposal(
		proposal.ID,
		chainID,
		15,
	); err != nil {
		t.Fatal(err)
	}

	if len(state.PendingProposals) != 0 {
		t.Fatalf(
			"expected no pending proposals after execution, got %d",
			len(state.PendingProposals),
		)
	}

	snapshot, err := state.Snapshot()
	if err != nil {
		t.Fatal(err)
	}

	if len(snapshot.PendingProposals) != 0 {
		t.Fatalf(
			"executed proposal remained in governance snapshot: %d",
			len(snapshot.PendingProposals),
		)
	}

	data, err := json.Marshal(snapshot)
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

	restored, err := GovernanceStateFromSnapshot(
		decoded,
	)
	if err != nil {
		t.Fatal(err)
	}

	if len(restored.PendingProposals) != 0 {
		t.Fatalf(
			"restored state resurrected executed proposal: %d",
			len(restored.PendingProposals),
		)
	}

	if _, exists := restored.GetPendingAuthorityProposal(
		proposal.ID,
	); exists {
		t.Fatal(
			"restored state resurrected executed proposal by ID",
		)
	}

	authorized, err := restored.CurrentPolicy.IsAuthorized(
		consensus.ReservedPoolTreasury,
		target.Address,
	)
	if err != nil {
		t.Fatal(err)
	}

	if !authorized {
		t.Fatal(
			"restored state lost executed authority change",
		)
	}

	err = restored.Replay.ValidateAuthorityChangeNext(
		proposal.Change,
	)

	if err == nil {
		t.Fatal(
			"restored replay state accepted executed authority change",
		)
	}

	if !strings.Contains(
		err.Error(),
		"already used",
	) {
		t.Fatalf(
			"unexpected replay error after restore: %v",
			err,
		)
	}
}
