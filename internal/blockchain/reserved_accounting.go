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

	authorityPolicy :=
		bc.Config.ReservedAuthorities

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

			continue
		}

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
	}

	return state, nil
}
