package main

import (
	"fmt"

	"prism/internal/blockchain"
	"prism/internal/reserved"
	"prism/internal/storage"
)

func loadOrCreateNodeGovernanceState(
	dataPath string,
	chain *blockchain.Blockchain,
) (
	*reserved.GovernanceState,
	bool,
	error,
) {
	if chain == nil {
		return nil,
			false,
			fmt.Errorf(
				"blockchain cannot be nil",
			)
	}

	chainID, err := chain.ChainID()
	if err != nil {
		return nil,
			false,
			fmt.Errorf(
				"cannot determine governance chain ID: %w",
				err,
			)
	}

	if storage.GovernanceStateExists(
		dataPath,
	) {
		state, err :=
			storage.LoadGovernanceState(
				dataPath,
			)

		if err != nil {
			return nil,
				false,
				fmt.Errorf(
					"cannot load reserved governance state: %w",
					err,
				)
		}

		if state.ChainID == "" {
			return nil,
				false,
				fmt.Errorf(
					"stored reserved governance state is not bound to a chain",
				)
		}

		if state.ChainID != chainID {
			return nil,
				false,
				fmt.Errorf(
					"reserved governance chain ID mismatch: stored=%s current=%s",
					state.ChainID,
					chainID,
				)
		}

		return state,
			false,
			nil
	}

	state, err :=
		reserved.NewGovernanceStateForChain(
			chainID,
			chain.Config.ReservedAuthorities,
		)

	if err != nil {
		return nil,
			false,
			fmt.Errorf(
				"cannot initialize reserved governance state: %w",
				err,
			)
	}

	if err := storage.SaveGovernanceState(
		dataPath,
		state,
	); err != nil {
		return nil,
			false,
			fmt.Errorf(
				"cannot persist initial reserved governance state: %w",
				err,
			)
	}

	return state,
		true,
		nil
}
