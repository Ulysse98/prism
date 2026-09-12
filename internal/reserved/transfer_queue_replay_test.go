package reserved

import (
	"testing"

	"prism/internal/consensus"
)

func TestValidateReservedTransferQueueReplayAcceptsFreshNonce(
	t *testing.T,
) {
	state :=
		NewAccountingState(0)

	proposal :=
		NewReservedTransferProposal(
			"prism-devnet",
			2,
			consensus.ReservedPoolTreasury,
			"recipient",
			100,
		)

	state.Replay.lastGrantNonce[grantReplayKey{
		Pool: consensus.ReservedPoolTreasury,
	}] = 1

	if err :=
		state.ValidateReservedTransferQueueReplay(
			proposal,
		); err != nil {

		t.Fatalf(
			"fresh reserved transfer nonce rejected: %v",
			err,
		)
	}
}

func TestValidateReservedTransferQueueReplayRejectsStaleNonce(
	t *testing.T,
) {
	state :=
		NewAccountingState(0)

	proposal :=
		NewReservedTransferProposal(
			"prism-devnet",
			1,
			consensus.ReservedPoolTreasury,
			"recipient",
			100,
		)

	state.Replay.lastGrantNonce[grantReplayKey{
		Pool: consensus.ReservedPoolTreasury,
	}] = 5

	if err :=
		state.ValidateReservedTransferQueueReplay(
			proposal,
		); err == nil {

		t.Fatal(
			"expected stale reserved transfer nonce to be rejected",
		)
	}
}

func TestValidateReservedTransferQueueReplayRejectsExecutedID(
	t *testing.T,
) {
	state :=
		NewAccountingState(0)

	proposal :=
		NewReservedTransferProposal(
			"prism-devnet",
			6,
			consensus.ReservedPoolTreasury,
			"recipient",
			100,
		)

	state.Replay.usedGrantIDs[proposal.ID] = struct{}{}

	if err :=
		state.ValidateReservedTransferQueueReplay(
			proposal,
		); err == nil {

		t.Fatal(
			"expected executed reserved transfer ID to be rejected",
		)
	}
}
