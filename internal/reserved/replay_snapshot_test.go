package reserved

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"prism/internal/consensus"
)

func TestReplaySnapshotJSONRoundTripPreservesState(
	t *testing.T,
) {
	state := NewReplayState()

	state.usedIDs["authorization-1"] =
		struct{}{}

	state.lastNonce[replayKey{
		Pool:       consensus.ReservedPoolTreasury,
		Authorizer: "authority-alice",
	}] = 7

	state.usedGrantIDs["grant-1"] =
		struct{}{}

	state.lastGrantNonce[grantReplayKey{
		Pool: consensus.ReservedPoolEcosystem,
	}] = 3

	state.revokedGrants[revocationReplayKey{
		Pool:    consensus.ReservedPoolTeam,
		GrantID: "grant-revoked-1",
	}] = struct{}{}

	state.usedAuthorityChangeIDs["authority-change-1"] =
		struct{}{}

	state.lastAuthorityChangeNonce[authorityChangeReplayKey{
		Pool: consensus.ReservedPoolLiquidity,
	}] = 11

	before, err := state.Snapshot()
	if err != nil {
		t.Fatal(err)
	}

	data, err := json.Marshal(before)
	if err != nil {
		t.Fatal(err)
	}

	var decoded ReplaySnapshot

	if err := json.Unmarshal(
		data,
		&decoded,
	); err != nil {
		t.Fatal(err)
	}

	restored, err :=
		ReplayStateFromSnapshot(decoded)

	if err != nil {
		t.Fatal(err)
	}

	after, err := restored.Snapshot()
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(
		before,
		after,
	) {
		t.Fatalf(
			"replay snapshot changed after JSON round trip:\nbefore=%+v\nafter=%+v",
			before,
			after,
		)
	}
}

func TestReplaySnapshotRestoreRejectsAuthorityChangeReplay(
	t *testing.T,
) {
	policy, authorities, target :=
		authorityChangePolicyFixture(t)

	change :=
		signedAuthorityChangeForReplay(
			t,
			policy,
			authorities[:2],
			4,
			consensus.ReservedPoolTreasury,
			AuthorityChangeAdd,
			target.Address,
		)

	state := NewReplayState()

	_, err :=
		state.AcceptAuthorityChange(
			change,
			policy,
			"prism-governance-replay-test",
		)

	if err != nil {
		t.Fatal(err)
	}

	snapshot, err := state.Snapshot()
	if err != nil {
		t.Fatal(err)
	}

	data, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatal(err)
	}

	var persisted ReplaySnapshot

	if err := json.Unmarshal(
		data,
		&persisted,
	); err != nil {
		t.Fatal(err)
	}

	restored, err :=
		ReplayStateFromSnapshot(
			persisted,
		)

	if err != nil {
		t.Fatal(err)
	}

	err =
		restored.ValidateAuthorityChangeNext(
			change,
		)

	if err == nil {
		t.Fatal(
			"expected restored replay state to reject authority change replay",
		)
	}

	if !strings.Contains(
		err.Error(),
		"already used",
	) {
		t.Fatalf(
			"expected already-used replay error after restore, got: %v",
			err,
		)
	}
}

func TestReplaySnapshotRejectsInvalidStoredPool(
	t *testing.T,
) {
	snapshot := ReplaySnapshot{
		AuthorityChangeNonces: []AuthorityChangeReplayRecord{
			{
				Pool: consensus.ReservedPool(
					"invalid-pool",
				),
				Nonce: 1,
			},
		},
	}

	_, err :=
		ReplayStateFromSnapshot(
			snapshot,
		)

	if err == nil {
		t.Fatal(
			"expected invalid stored pool to fail",
		)
	}

	if !strings.Contains(
		err.Error(),
		"invalid stored reserved pool",
	) {
		t.Fatalf(
			"unexpected invalid pool error: %v",
			err,
		)
	}
}
