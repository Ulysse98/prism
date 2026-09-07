package reserved

import (
	"fmt"
	"math"

	"prism/internal/consensus"
)

type AccountingState struct {
	Usage  consensus.ReservedUsage
	Replay *ReplayState
}

func NewAccountingState(
	legacyGenesis uint64,
) *AccountingState {
	return &AccountingState{
		Usage: consensus.ReservedUsage{
			LegacyGenesis: legacyGenesis,
		},
		Replay: NewReplayState(),
	}
}

func (state *AccountingState) Budget(
	policy consensus.SupplyPolicy,
) (
	consensus.ReservedBudgetState,
	error,
) {
	if state == nil {
		return consensus.ReservedBudgetState{},
			fmt.Errorf(
				"reserved accounting state cannot be nil",
			)
	}

	return state.Usage.Remaining(policy)
}

func addAuthorizationUsage(
	usage consensus.ReservedUsage,
	authorization Authorization,
) (
	consensus.ReservedUsage,
	error,
) {
	amount := authorization.Amount

	switch authorization.Pool {
	case consensus.ReservedPoolEcosystem:
		if usage.Ecosystem > math.MaxUint64-amount {
			return consensus.ReservedUsage{},
				fmt.Errorf(
					"ecosystem reserved usage overflow",
				)
		}
		usage.Ecosystem += amount

	case consensus.ReservedPoolTreasury:
		if usage.Treasury > math.MaxUint64-amount {
			return consensus.ReservedUsage{},
				fmt.Errorf(
					"treasury reserved usage overflow",
				)
		}
		usage.Treasury += amount

	case consensus.ReservedPoolTeam:
		if usage.Team > math.MaxUint64-amount {
			return consensus.ReservedUsage{},
				fmt.Errorf(
					"team reserved usage overflow",
				)
		}
		usage.Team += amount

	case consensus.ReservedPoolLiquidity:
		if usage.Liquidity > math.MaxUint64-amount {
			return consensus.ReservedUsage{},
				fmt.Errorf(
					"liquidity reserved usage overflow",
				)
		}
		usage.Liquidity += amount

	default:
		return consensus.ReservedUsage{},
			fmt.Errorf(
				"unknown reserved pool: %q",
				authorization.Pool,
			)
	}

	return usage, nil
}

func (state *AccountingState) Accept(
	authorization Authorization,
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
		authorityPolicy.ValidateAuthorization(
			authorization,
			expectedChainID,
		); err != nil {

		return err
	}

	if err :=
		replay.ValidateNext(
			authorization,
		); err != nil {

		return err
	}

	nextUsage, err :=
		addAuthorizationUsage(
			state.Usage,
			authorization,
		)

	if err != nil {
		return err
	}

	if err :=
		nextUsage.Validate(
			supplyPolicy,
		); err != nil {

		return fmt.Errorf(
			"reserved budget rejected authorization: %w",
			err,
		)
	}

	if err :=
		replay.Accept(
			authorization,
			authorityPolicy,
			expectedChainID,
		); err != nil {

		return err
	}

	state.Usage = nextUsage
	state.Replay = replay

	return nil
}
