package reserved

import (
	"fmt"

	"prism/internal/consensus"
)

func (state *AccountingState) AcceptGrant(
	grant Grant,
	authorityPolicy AuthorityPolicy,
	expectedChainID string,
	supplyPolicy consensus.SupplyPolicy,
) error {
	return state.AcceptGrantAtHeight(
		grant,
		0,
		authorityPolicy,
		expectedChainID,
		supplyPolicy,
	)
}

func (state *AccountingState) AcceptGrantAtHeight(
	grant Grant,
	blockHeight uint64,
	authorityPolicy AuthorityPolicy,
	expectedChainID string,
	supplyPolicy consensus.SupplyPolicy,
) error {
	if state == nil {
		return fmt.Errorf(
			"reserved accounting state cannot be nil",
		)
	}

	if err := supplyPolicy.Validate(); err != nil {
		return fmt.Errorf(
			"invalid supply policy: %w",
			err,
		)
	}

	replay := state.Replay

	if replay == nil {
		replay = NewReplayState()
	}

	if err :=
		authorityPolicy.ValidateGrant(
			grant,
			expectedChainID,
		); err != nil {

		return err
	}

	if err :=
		ValidateGrantAtHeight(
			grant,
			blockHeight,
		); err != nil {

		return err
	}

	if err :=
		replay.ValidateGrantNext(
			grant,
		); err != nil {

		return err
	}

	nextUsage, err :=
		addAuthorizationUsage(
			state.Usage,
			Authorization{
				Pool:   grant.Pool,
				Amount: grant.Amount,
			},
		)

	if err != nil {
		return err
	}

	if err :=
		nextUsage.Validate(
			supplyPolicy,
		); err != nil {

		return fmt.Errorf(
			"reserved budget rejected grant: %w",
			err,
		)
	}

	if err := replay.AcceptGrant(grant); err != nil {
		return err
	}

	state.Usage = nextUsage
	state.Replay = replay

	return nil
}
