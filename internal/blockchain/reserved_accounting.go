package blockchain

import (
	"fmt"

	"prism/internal/consensus"
	"prism/internal/reserved"
)

func (bc *Blockchain) GetReservedAccountingState() (
	*reserved.AccountingState,
	error,
) {
	if bc == nil {
		return nil,
			fmt.Errorf(
				"blockchain cannot be nil",
			)
	}
	if bc.hasReservedTransferGovernance() {
		_, accounting, err :=
			bc.replayReservedTransferConsensusState()

		if err != nil {
			return nil, err
		}

		return accounting, nil
	}

	genesisSupply, err :=
		bc.GenesisSupply()

	if err != nil {
		return nil,
			fmt.Errorf(
				"cannot determine genesis supply: %w",
				err,
			)
	}

	chainID, err :=
		bc.ChainID()

	if err != nil {
		return nil,
			fmt.Errorf(
				"cannot determine chain identity: %w",
				err,
			)
	}

	supplyPolicy :=
		consensus.DefaultSupplyPolicy()

	if err :=
		supplyPolicy.Validate(); err != nil {

		return nil,
			fmt.Errorf(
				"invalid supply policy: %w",
				err,
			)
	}

	state :=
		reserved.NewAccountingState(
			genesisSupply,
		)

	governance, err :=
		reserved.NewGovernanceStateForChain(
			chainID,
			bc.Config.ReservedAuthorities,
		)

	if err != nil {
		return nil,
			fmt.Errorf(
				"cannot initialize reserved governance state: %w",
				err,
			)
	}

	for blockIndex, block := range bc.Blocks {
		if blockIndex == 0 {
			if len(
				block.ReservedAuthorizations,
			) != 0 {

				return nil,
					fmt.Errorf(
						"genesis block cannot contain reserved authorizations",
					)
			}

			if len(
				block.ReservedGrants,
			) != 0 {

				return nil,
					fmt.Errorf(
						"genesis block cannot contain reserved grants",
					)
			}

			if len(
				block.ReservedRevocations,
			) != 0 {

				return nil,
					fmt.Errorf(
						"genesis block cannot contain reserved revocations",
					)
			}

			if len(
				block.AuthorityChanges,
			) != 0 {

				return nil,
					fmt.Errorf(
						"genesis block cannot contain reserved authority changes",
					)
			}

			continue
		}

		// Reserved operations in a block are validated against
		// the authority policy active at the beginning of that
		// block.
		//
		// Authority changes contained in the same block become
		// active only after those operations have been replayed.
		// This prevents a newly-added authority from authorizing
		// an operation in the block that grants it authority.
		authorityPolicy :=
			governance.CurrentPolicy

		for authorizationIndex, authorization := range block.ReservedAuthorizations {

			if err :=
				state.Accept(
					authorization,
					authorityPolicy,
					chainID,
					supplyPolicy,
				); err != nil {

				return nil,
					fmt.Errorf(
						"invalid reserved authorization in block %d at index %d: %w",
						block.Height,
						authorizationIndex,
						err,
					)
			}
		}

		for revocationIndex, revocation := range block.ReservedRevocations {

			if err :=
				state.AcceptRevocation(
					revocation,
					authorityPolicy,
					chainID,
				); err != nil {

				return nil,
					fmt.Errorf(
						"invalid reserved revocation in block %d at index %d: %w",
						block.Height,
						revocationIndex,
						err,
					)
			}
		}

		for grantIndex, grant := range block.ReservedGrants {

			if err :=
				state.AcceptGrantAtHeight(
					grant,
					block.Height,
					authorityPolicy,
					chainID,
					supplyPolicy,
				); err != nil {

				return nil,
					fmt.Errorf(
						"invalid reserved grant in block %d at index %d: %w",
						block.Height,
						grantIndex,
						err,
					)
			}
		}

		// Governance changes are applied after all reserved
		// operations in the current block. The resulting policy
		// becomes active for the following block.
		for changeIndex, change := range block.AuthorityChanges {

			if err :=
				governance.ApplyAuthorityChange(
					change,
					chainID,
				); err != nil {

				return nil,
					fmt.Errorf(
						"invalid reserved authority change in block %d at index %d: %w",
						block.Height,
						changeIndex,
						err,
					)
			}
		}
	}

	return state, nil
}
